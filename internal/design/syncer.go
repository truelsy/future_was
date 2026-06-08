package design

import (
	"context"
	"errors"
	"fmt"
	"future_was/internal/util"
	"sync"

	"future_was/internal/log"
	"future_was/internal/repository"

	"github.com/redis/go-redis/v9"
)

// PubSubChannel 디자인 reload 알림용 Redis Pub/Sub 채널.
const PubSubChannel = "design:reload"

// MaxActiveVersions LoadActive에서 메모리에 유지할 server_version 최대 개수.
const MaxActiveVersions = 2

// Syncer TB_VERSION 기반 디자인 버전 자동 로드 + 멀티 서버 동기화.
// is_active=1인 server_version 중 최신 MaxActiveVersions개를 메모리에 유지한다.
//
// Pub/Sub 동작:
//   - Trigger 받은 서버: 즉시 LoadActive (caller 가 fresh 응답 받도록) + selfID 를 페이로드로 publish
//   - subscribe: 페이로드가 자기 selfID 면 skip (Redis 가 publisher 본인에게도 broadcast 하기 때문)
//     → publisher 측 중복 다운로드 방지
type Syncer struct {
	store    *Store
	loader   *Loader
	redis    *redis.Client
	versions *repository.VersionRepository
	selfID   string // 프로세스 생애 1회 발급. self-published 메시지 dedup 용
	wg       sync.WaitGroup
}

func NewSyncer(store *Store, loader *Loader, redisClient *redis.Client, versions *repository.VersionRepository) *Syncer {
	return &Syncer{
		store:    store,
		loader:   loader,
		redis:    redisClient,
		versions: versions,
		selfID:   util.NewInstanceID(),
	}
}

// Start 백그라운드 고루틴으로 Pub/Sub 구독을 시작한다.
func (s *Syncer) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.subscribe(ctx)
	}()
}

// Wait subscribe 고루틴이 종료될 때까지 블록한다.
// 호출자는 ctx cancel 후 이 메서드로 정리 완료를 보장받는다.
func (s *Syncer) Wait() {
	s.wg.Wait()
}

// Trigger TB_VERSION 기반으로 본 서버를 갱신하고 다른 서버에 알린다.
// 페이로드에 selfID 를 실어 보내 subscribe 쪽에서 self-message 를 dedup.
func (s *Syncer) Trigger(ctx context.Context) error {
	if err := s.LoadActive(ctx); err != nil {
		return err
	}
	return s.redis.Publish(ctx, PubSubChannel, s.selfID).Err()
}

// LoadActive TB_VERSION에서 is_active=1인 행을 조회하여,
// 최신 N개 server_version의 Catalog을 로드하고 client_version → Catalog 매핑을 갱신한다.
// 호출될 때마다 CDN에서 항상 다시 받는다 — 같은 server_version 내 파일 변경(JSON 재업로드,
// 새 시트 추가 등)을 일관되게 반영하기 위함. Trigger 는 빈도가 낮아 (운영자 reload / Pub/Sub
// 알림) CDN 비용 부담은 무시 가능.
func (s *Syncer) LoadActive(ctx context.Context) error {
	rows, err := s.versions.FindActiveOrderedByServerVersion()
	if err != nil {
		return fmt.Errorf("query TB_VERSION: %w", err)
	}

	// server_version 정렬 순서 보존 + server_version별 client_version 그룹핑.
	var orderedSV []string
	seen := map[string]bool{}
	cvBySV := map[string][]string{}
	for _, v := range rows {
		if !seen[v.ServerVersion] {
			seen[v.ServerVersion] = true
			orderedSV = append(orderedSV, v.ServerVersion)
		}
		cvBySV[v.ServerVersion] = append(cvBySV[v.ServerVersion], v.ClientVersion)
	}

	if len(orderedSV) == 0 {
		return errors.New("no active version in TB_VERSION")
	}

	// 최신 MaxActiveVersions개만 유지.
	if len(orderedSV) > MaxActiveVersions {
		orderedSV = orderedSV[:MaxActiveVersions]
	}

	newMap := map[string]*Catalog{}
	for i, sv := range orderedSV {
		catalog, err := s.loader.Load(ctx, sv)
		if err != nil {
			// 최신 버전 로드 실패는 치명적, 그 외는 경고만.
			if i == 0 {
				return fmt.Errorf("load latest %s: %w", sv, err)
			}
			log.Warn().Err(err).Msgf("design load skipped: %s", sv)
			continue
		}
		log.Info().Msgf("design loaded: %s", sv)
		for _, cv := range cvBySV[sv] {
			newMap[cv] = catalog
		}
	}

	s.store.Replace(newMap)
	log.Info().Msgf("design active versions: %v", orderedSV)
	return nil
}

// subscribe 다른 서버의 PUBLISH를 수신하여 자기 메모리도 갱신한다.
// 페이로드가 자기 selfID 와 일치하면 본인이 publish 한 메시지이므로 skip
// (Trigger 에서 이미 LoadActive 했으니 중복 다운로드 회피).
func (s *Syncer) subscribe(ctx context.Context) {
	pubsub := s.redis.Subscribe(ctx, PubSubChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg.Payload == s.selfID && s.selfID != "" {
				continue // self-published — Trigger 가 이미 처리함
			}
			if err := s.LoadActive(ctx); err != nil {
				log.Error().Err(err).Msg("design reload failed (subscriber)")
			}
		}
	}
}

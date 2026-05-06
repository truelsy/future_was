package design

import (
	"context"
	"errors"
	"fmt"

	"future_next_baseball/internal/log"
	"future_next_baseball/internal/repository"

	"github.com/redis/go-redis/v9"
)

// PubSubChannel 디자인 reload 알림용 Redis Pub/Sub 채널.
const PubSubChannel = "design:reload"

// reloadPayload Pub/Sub 메시지 페이로드. 단순 트리거 신호.
const reloadPayload = "reload"

// Syncer TB_VERSION 기반 디자인 버전 자동 로드 + 멀티 서버 동기화.
// Trigger는 본인 서버 갱신 + 다른 서버에 PUBLISH.
// Start로 백그라운드 SUBSCRIBE 시작.
type Syncer struct {
	store    *Store
	loader   *Loader
	redis    *redis.Client
	versions *repository.VersionRepository
}

func NewSyncer(store *Store, loader *Loader, redisClient *redis.Client, versions *repository.VersionRepository) *Syncer {
	return &Syncer{
		store:    store,
		loader:   loader,
		redis:    redisClient,
		versions: versions,
	}
}

// Start 백그라운드 고루틴으로 Pub/Sub 구독을 시작한다.
func (s *Syncer) Start(ctx context.Context) {
	go s.subscribe(ctx)
}

// Trigger TB_VERSION 기반으로 본 서버를 갱신하고 다른 서버에 알린다.
func (s *Syncer) Trigger(ctx context.Context) error {
	if err := s.LoadActive(ctx); err != nil {
		return err
	}
	return s.redis.Publish(ctx, PubSubChannel, reloadPayload).Err()
}

// LoadActive TB_VERSION에서 is_active=1인 행을 조회하여
// 최신 server_version → current, 그 다음 → previous로 로드한다.
// versionMap (client_version → server_version)도 갱신한다.
func (s *Syncer) LoadActive(ctx context.Context) error {
	rows, err := s.versions.FindActiveOrderedByServerVersion()
	if err != nil {
		return fmt.Errorf("query TB_VERSION: %w", err)
	}

	// server_version 정렬 순서 보존하면서 중복 제거 + client_version 매핑 구성.
	var orderedSV []string
	seen := map[string]bool{}
	versionMap := map[string]string{}
	for _, v := range rows {
		if !seen[v.ServerVersion] {
			seen[v.ServerVersion] = true
			orderedSV = append(orderedSV, v.ServerVersion)
		}
		versionMap[v.ClientVersion] = v.ServerVersion
	}

	if len(orderedSV) == 0 {
		return errors.New("no active version in TB_VERSION")
	}

	// 1) 최신 → current
	currentSV := orderedSV[0]
	if s.store.CurrentVersion() != currentSV {
		snap, err := s.loader.Load(ctx, currentSV)
		if err != nil {
			return fmt.Errorf("load current %s: %w", currentSV, err)
		}
		s.store.Promote(snap)
		log.Info().Msgf("design current loaded: %s", currentSV)
	}

	// 2) 직전 → previous (있는 경우만)
	// CDN에 해당 버전 데이터가 없어도 서비스를 막지 않는다 (로그만 남김).
	if len(orderedSV) >= 2 {
		prevSV := orderedSV[1]
		if s.store.PreviousVersion() != prevSV {
			snap, err := s.loader.Load(ctx, prevSV)
			if err != nil {
				log.Warn().Err(err).Msgf("design previous load skipped: %s", prevSV)
			} else {
				s.store.SetPrevious(snap)
				log.Info().Msgf("design previous loaded: %s", prevSV)
			}
		}
	}

	s.store.SetVersionMap(versionMap)
	return nil
}

// subscribe 다른 서버의 PUBLISH를 수신하여 자기 메모리도 갱신한다.
// 페이로드 무시, 항상 DB에서 다시 조회.
func (s *Syncer) subscribe(ctx context.Context) {
	pubsub := s.redis.Subscribe(ctx, PubSubChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			if err := s.LoadActive(ctx); err != nil {
				log.Error().Err(err).Msg("design reload failed (subscriber)")
			}
		}
	}
}

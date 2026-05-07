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

// MaxActiveVersions LoadActive에서 메모리에 유지할 server_version 최대 개수.
const MaxActiveVersions = 2

// Syncer TB_VERSION 기반 디자인 버전 자동 로드 + 멀티 서버 동기화.
// is_active=1인 server_version 중 최신 MaxActiveVersions개를 메모리에 유지한다.
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

// LoadActive TB_VERSION에서 is_active=1인 행을 조회하여,
// 최신 N개 server_version의 Catalog을 로드하고 client_version → Catalog 매핑을 갱신한다.
// 기존에 로드된 Catalog은 재사용한다 (CDN 재다운로드 회피).
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

	// 기존 Catalog 재사용 (CDN 재다운로드 회피).
	existing := s.store.CatalogsByServerVersion()

	newMap := map[string]*Catalog{}
	for i, sv := range orderedSV {
		catalog, ok := existing[sv]
		if !ok {
			catalog, err = s.loader.Load(ctx, sv)
			if err != nil {
				// 최신 버전 로드 실패는 치명적, 그 외는 경고만.
				if i == 0 {
					return fmt.Errorf("load latest %s: %w", sv, err)
				}
				log.Warn().Err(err).Msgf("design load skipped: %s", sv)
				continue
			}
			log.Info().Msgf("design loaded: %s", sv)
		}
		for _, cv := range cvBySV[sv] {
			newMap[cv] = catalog
		}
	}

	s.store.Replace(newMap)
	log.Info().Msgf("design active versions: %v", orderedSV)
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

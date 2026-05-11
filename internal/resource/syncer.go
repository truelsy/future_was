package resource

import (
	"context"
	"future_next_baseball/internal/log"
	"sync"

	"github.com/redis/go-redis/v9"
)

// PubSubChannel 리소스 reload 알림용 Redis Pub/Sub 채널.
const PubSubChannel = "resource:reload"

// reloadPayload Pub/Sub 메시지 페이로드. 단순 트리거 신호.
const reloadPayload = "reload"

type Syncer struct {
	store  *Store
	loader *Loader
	redis  *redis.Client
	wg     sync.WaitGroup
}

func NewSyncer(store *Store, loader *Loader, redisClient *redis.Client) *Syncer {
	return &Syncer{
		store:  store,
		loader: loader,
		redis:  redisClient,
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

// LoadAll DB 조회 → Snapshot 원자 교체.
func (s *Syncer) LoadAll(ctx context.Context) error {
	snap, err := s.loader.Load(ctx)
	if err != nil {
		return err
	}
	s.store.Replace(snap)
	log.Info().Int("maintenance", len(snap.maintenance)).Msg("resource data loaded")
	return nil
}

// Trigger 본 서버 갱신 + 다른 서버에 알림.
func (s *Syncer) Trigger(ctx context.Context) error {
	if err := s.LoadAll(ctx); err != nil {
		return err
	}
	return s.redis.Publish(ctx, PubSubChannel, reloadPayload).Err()
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
			if err := s.LoadAll(ctx); err != nil {
				log.Error().Err(err).Msg("resource reload failed (subscriber)")
			}
		}
	}
}

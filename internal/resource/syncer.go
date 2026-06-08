package resource

import (
	"context"
	"future_was/internal/log"
	"future_was/internal/util"
	"sync"

	"github.com/redis/go-redis/v9"
)

// PubSubChannel 리소스 reload 알림용 Redis Pub/Sub 채널.
const PubSubChannel = "resource:reload"

// Syncer DB 기반 리소스 데이터 메모리 보관 + 멀티 서버 동기화.
//
// Pub/Sub 동작:
//   - Trigger 받은 서버: 즉시 LoadAll + selfID 페이로드로 publish
//   - subscribe: 페이로드가 자기 selfID 면 skip (publisher 본인 중복 처리 방지)
type Syncer struct {
	store  *Store
	loader *Loader
	redis  *redis.Client
	selfID string // 프로세스 생애 1회 발급. self-published 메시지 dedup 용
	wg     sync.WaitGroup
}

func NewSyncer(store *Store, loader *Loader, redisClient *redis.Client) *Syncer {
	return &Syncer{
		store:  store,
		loader: loader,
		redis:  redisClient,
		selfID: util.NewInstanceID(),
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
// 페이로드에 selfID 를 실어 subscribe 쪽에서 self-message dedup.
func (s *Syncer) Trigger(ctx context.Context) error {
	if err := s.LoadAll(ctx); err != nil {
		return err
	}
	return s.redis.Publish(ctx, PubSubChannel, s.selfID).Err()
}

// subscribe 다른 서버의 PUBLISH를 수신하여 자기 메모리도 갱신한다.
// 페이로드가 자기 selfID 면 본인이 publish 한 메시지이므로 skip.
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
			if err := s.LoadAll(ctx); err != nil {
				log.Error().Err(err).Msg("resource reload failed (subscriber)")
			}
		}
	}
}

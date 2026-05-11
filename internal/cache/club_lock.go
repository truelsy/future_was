package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ClubLock Redis 기반 클럽 단위 분산락.
// 멤버 N명이 동시에 같은 클럽을 변경하는 상황을 직렬화한다.
type ClubLock struct {
	client *redis.Client
}

const (
	clubLockKeyPrefix  = "lock:club:"
	clubLockTTL        = 10 * time.Second
	clubLockRetryDelay = 50 * time.Millisecond
	clubLockMaxRetries = 60 // 50ms × 60 = 약 3초
)

// ErrClubLockAcquireTimeout 재시도 횟수 초과 시 반환.
var ErrClubLockAcquireTimeout = errors.New("clublock: acquire timeout")

// releaseScript는 user_lock.go의 것을 재사용한다.

func NewClubLock() *ClubLock {
	return &ClubLock{client: Get(NameLock)}
}

func clubLockKey(clubID uint64) string {
	return fmt.Sprintf("%s%d", clubLockKeyPrefix, clubID)
}

// Acquire 클럽 락 획득. 점유 중이면 짧게 대기 후 재시도.
// 데드락 회피를 위해 user 락 획득 후에 호출하는 컨벤션을 지켜야 한다.
func (l *ClubLock) Acquire(ctx context.Context, clubID uint64) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}

	key := clubLockKey(clubID)
	args := redis.SetArgs{Mode: "NX", TTL: clubLockTTL}
	for range clubLockMaxRetries {
		_, err := l.client.SetArgs(ctx, key, token, args).Result()
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, redis.Nil) {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(clubLockRetryDelay):
		}
	}
	return "", ErrClubLockAcquireTimeout
}

// Release 토큰 매칭 시에만 락을 해제한다 (다른 클라이언트의 락 보호).
func (l *ClubLock) Release(ctx context.Context, clubID uint64, token string) error {
	keys := []string{clubLockKey(clubID)}
	return releaseScript.Run(ctx, l.client, keys, token).Err()
}

package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// UserLock Redis 기반 유저 단위 분산락. 멀티 서버 환경에서
// 동일 유저의 동시 요청을 직렬화한다.
type UserLock struct {
	client *redis.Client
}

const (
	userLockKeyPrefix  = "lock:user:"
	userLockTTL        = 10 * time.Second
	userLockRetryDelay = 50 * time.Millisecond
	userLockMaxRetries = 60 // 50ms × 60 = 약 3초
)

// ErrLockAcquireTimeout 재시도 횟수를 초과하여 락 획득에 실패했을 때 반환된다.
var ErrLockAcquireTimeout = errors.New("userlock: acquire timeout")

// releaseScript 토큰이 일치할 때만 락을 삭제한다 (atomic check-and-del).
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`)

func NewUserLock() *UserLock {
	return &UserLock{client: Get(NameUserLock)}
}

func userLockKey(userID uint64) string {
	return fmt.Sprintf("%s%d", userLockKeyPrefix, userID)
}

// Acquire 유저 락을 획득한다. 이미 잠겨 있으면 짧게 대기 후 재시도.
// 토큰을 반환하며, Release 시 동일 토큰으로만 해제 가능하다.
func (l *UserLock) Acquire(ctx context.Context, userID uint64) (string, error) {
	// 토큰 생성
	token, err := newToken()
	if err != nil {
		return "", err
	}

	key := userLockKey(userID)
	args := redis.SetArgs{Mode: "NX", TTL: userLockTTL}
	for range userLockMaxRetries {
		_, err := l.client.SetArgs(ctx, key, token, args).Result()

		// 성공 (Lock 획득)
		if err == nil {
			return token, nil
		}

		// 네트워크 or Redis에러는 즉시 실패 처리
		if !errors.Is(err, redis.Nil) {
			return "", err
		}

		// Lock 점유중인 경우 재시도
		select {
		case <-ctx.Done(): //  클라이언트 연결 끊김, 요청 타임아웃 등으로 context가 취소되면 즉시 종료
			return "", ctx.Err()
		case <-time.After(userLockRetryDelay): // 50ms 후 재시도
		}
	}
	return "", ErrLockAcquireTimeout
}

// Release Acquire에서 받은 토큰으로 락을 해제한다.
// 토큰이 일치할 때만 삭제되어, 다른 클라이언트의 락을 실수로 풀지 않는다.
func (l *UserLock) Release(ctx context.Context, userID uint64, token string) error {
	keys := []string{userLockKey(userID)}
	return releaseScript.Run(ctx, l.client, keys, token).Err()
}

// newToken 32바이트 hex 문자열을 생성한다.
func newToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

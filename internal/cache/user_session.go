package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sessionKeyPrefix = "session:user:"
	sessionTTL       = 8 * time.Hour
)

// verifyAndRefreshScript 저장된 토큰이 일치할 때만 TTL을 갱신한다.
//
//	KEYS[1] = session:user:{id}
//	ARGV[1] = client_token
//	ARGV[2] = ttl_seconds
//
// 반환: 1=일치/갱신, 0=불일치 또는 미존재.
var verifyAndRefreshScript = redis.NewScript(`
local stored = redis.call("GET", KEYS[1])
if stored == ARGV[1] then
    redis.call("EXPIRE", KEYS[1], tonumber(ARGV[2]))
    return 1
end
return 0
`)

// UserSession Redis 기반 유저 세션 저장소.
// Login 시 토큰을 발급(Set)하고, 디스패처가 매 요청 검증(VerifyAndRefresh)한다.
type UserSession struct {
	client *redis.Client
}

func NewUserSession() *UserSession {
	return &UserSession{client: Get(NameUserSession)}
}

func sessionKey(userID uint64) string {
	return fmt.Sprintf("%s%d", sessionKeyPrefix, userID)
}

// Set 새 토큰을 발급해 8시간 TTL로 저장하고 반환한다. 기존 세션은 덮어쓴다.
func (s *UserSession) Set(userID uint64) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	if err := s.client.Set(ctx, sessionKey(userID), token, sessionTTL).Err(); err != nil {
		return "", fmt.Errorf("user session set: %w", err)
	}
	return token, nil
}

// VerifyAndRefresh 토큰이 일치하면 TTL을 sliding 갱신하고 true를 반환한다.
// 미존재·불일치는 false (에러 아님). Redis 통신 실패만 err로 반환된다.
func (s *UserSession) VerifyAndRefresh(ctx context.Context, userID uint64, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	ttl := int(sessionTTL.Seconds())
	res, err := verifyAndRefreshScript.Run(ctx, s.client, []string{sessionKey(userID)}, token, ttl).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// Delete 명시적 로그아웃 시 세션을 제거한다.
func (s *UserSession) Delete(userID uint64) error {
	ctx := context.Background()
	return s.client.Del(ctx, sessionKey(userID)).Err()
}

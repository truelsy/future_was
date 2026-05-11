package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	userKeyPrefix = "user:"
	defaultTTL    = 30 * time.Minute
)

// hsetWithTTLScript HSET + EXPIRE를 원자 실행한다.
// 두 명령 사이 장애로 TTL 없는 Hash가 영구 잔존하는 상황을 방지한다.
//   KEYS[1]   = key
//   ARGV[1]   = ttl_seconds
//   ARGV[2..] = field, value, field, value, ... (페어로 반복)
var hsetWithTTLScript = redis.NewScript(`
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
for i = 2, #ARGV, 2 do
    redis.call("HSET", key, ARGV[i], ARGV[i+1])
end
redis.call("EXPIRE", key, ttl)
return 1
`)

// UserCache Redis Hash를 사용하여 유저별 캐시 데이터를 관리한다.
type UserCache struct {
	client *redis.Client
}

func NewUserCache() *UserCache {
	return &UserCache{client: Get(NameUserCache)}
}

func userKey(userID uint64) string {
	return fmt.Sprintf("%s%d", userKeyPrefix, userID)
}

// Exists 유저 캐시가 존재하는지 확인한다.
func (c *UserCache) Exists(userID uint64) (bool, error) {
	ctx := context.Background()
	n, err := c.client.Exists(ctx, userKey(userID)).Result()
	return n > 0, err
}

// Get 유저 캐시에서 특정 필드를 조회하여 dest에 역직렬화한다.
func (c *UserCache) Get(userID uint64, field string, dest any) error {
	ctx := context.Background()
	data, err := c.client.HGet(ctx, userKey(userID), field).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// Set 유저 캐시에 특정 필드를 저장하고 TTL을 갱신한다.
// HSET + EXPIRE를 Lua 스크립트로 원자 실행한다.
func (c *UserCache) Set(userID uint64, field string, value any) error {
	ctx := context.Background()
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %w", err)
	}

	key := userKey(userID)
	ttl := int(defaultTTL.Seconds())
	return hsetWithTTLScript.Run(ctx, c.client, []string{key}, ttl, field, data).Err()
}

// SetMulti 유저 캐시에 여러 필드를 한번에 저장하고 TTL을 설정한다.
// 모든 HSET + EXPIRE를 Lua 스크립트로 원자 실행한다.
func (c *UserCache) SetMulti(userID uint64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}

	ctx := context.Background()
	key := userKey(userID)

	// ARGV: [ttl, field1, value1, field2, value2, ...]
	args := make([]any, 0, 1+len(fields)*2)
	args = append(args, int(defaultTTL.Seconds()))
	for field, v := range fields {
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal cache field [%s]: %w", field, err)
		}
		args = append(args, field, data)
	}

	return hsetWithTTLScript.Run(ctx, c.client, []string{key}, args...).Err()
}

// LoadAll 유저 캐시의 모든 필드를 원시 JSON 문자열로 반환한다.
func (c *UserCache) LoadAll(userID uint64) (map[string]string, error) {
	ctx := context.Background()
	return c.client.HGetAll(ctx, userKey(userID)).Result()
}

// DeleteField 유저 캐시에서 특정 필드를 삭제한다.
func (c *UserCache) DeleteField(userID uint64, field string) error {
	ctx := context.Background()
	return c.client.HDel(ctx, userKey(userID), field).Err()
}

// DeleteAll 유저 캐시 전체를 삭제한다.
func (c *UserCache) DeleteAll(userID uint64) error {
	ctx := context.Background()
	return c.client.Del(ctx, userKey(userID)).Err()
}

// Refresh 유저 캐시의 TTL을 리셋한다.
func (c *UserCache) Refresh(userID uint64) error {
	ctx := context.Background()
	return c.client.Expire(ctx, userKey(userID), defaultTTL).Err()
}

// GetOrLoad 캐시를 먼저 조회한다. 캐시 미스 시 loader를 호출하여 DB에서 가져오고,
// 결과를 캐시에 저장한 뒤 반환한다.
// 서비스에서 캐시 로직을 신경 쓰지 않고 사용할 수 있다.
func GetOrLoad[T any](c *UserCache, userID uint64, field string, loader func() (T, error)) (T, error) {
	var result T

	// 1. 캐시 히트
	if err := c.Get(userID, field, &result); err == nil {
		return result, nil
	}

	// 2. Cache Miss → DB 조회
	loaded, err := loader()
	if err != nil {
		var zero T
		return zero, err
	}

	// 3. 캐시에 저장
	_ = c.Set(userID, field, loaded)
	return loaded, nil
}

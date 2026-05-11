package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	clubKeyPrefix = "club:"
	clubTTL       = 1 * time.Minute // 멤버 간 stale 최소화 위해 user(30분)보다 짧게
)

// ClubCache Redis Hash를 사용하여 클럽별 캐시 데이터를 관리한다.
// 키 prefix와 TTL만 다르며, 동작은 UserCache와 동일하다.
// hsetWithTTLScript는 user_cache.go의 것을 재사용한다.
type ClubCache struct {
	client *redis.Client
}

func NewClubCache() *ClubCache {
	return &ClubCache{client: Get(NameClubCache)}
}

func clubKey(clubID uint64) string {
	return fmt.Sprintf("%s%d", clubKeyPrefix, clubID)
}

// Exists 클럽 캐시 존재 여부.
func (c *ClubCache) Exists(clubID uint64) (bool, error) {
	ctx := context.Background()
	n, err := c.client.Exists(ctx, clubKey(clubID)).Result()
	return n > 0, err
}

// Get 특정 필드 조회 + dest로 역직렬화.
func (c *ClubCache) Get(clubID uint64, field string, dest any) error {
	ctx := context.Background()
	data, err := c.client.HGet(ctx, clubKey(clubID), field).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// Set 단일 필드 저장 + TTL 원자 적용.
func (c *ClubCache) Set(clubID uint64, field string, value any) error {
	ctx := context.Background()
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal club cache value: %w", err)
	}
	key := clubKey(clubID)
	ttl := int(clubTTL.Seconds())
	return hsetWithTTLScript.Run(ctx, c.client, []string{key}, ttl, field, data).Err()
}

// SetMulti 여러 필드 일괄 저장 + TTL 원자 적용.
func (c *ClubCache) SetMulti(clubID uint64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	ctx := context.Background()
	key := clubKey(clubID)

	args := make([]any, 0, 1+len(fields)*2)
	args = append(args, int(clubTTL.Seconds()))
	for field, v := range fields {
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal club cache field [%s]: %w", field, err)
		}
		args = append(args, field, data)
	}

	return hsetWithTTLScript.Run(ctx, c.client, []string{key}, args...).Err()
}

// LoadAll 모든 필드를 raw JSON 문자열로 반환.
func (c *ClubCache) LoadAll(clubID uint64) (map[string]string, error) {
	ctx := context.Background()
	return c.client.HGetAll(ctx, clubKey(clubID)).Result()
}

// DeleteField 특정 필드 삭제.
func (c *ClubCache) DeleteField(clubID uint64, field string) error {
	ctx := context.Background()
	return c.client.HDel(ctx, clubKey(clubID), field).Err()
}

// DeleteAll 클럽 캐시 전체 삭제.
func (c *ClubCache) DeleteAll(clubID uint64) error {
	ctx := context.Background()
	return c.client.Del(ctx, clubKey(clubID)).Err()
}

// Refresh TTL 리셋.
func (c *ClubCache) Refresh(clubID uint64) error {
	ctx := context.Background()
	return c.client.Expire(ctx, clubKey(clubID), clubTTL).Err()
}


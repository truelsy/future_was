package cache

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	instances = make(map[string]*redis.Client)
	mu        sync.RWMutex
)

// Init 이름 기반으로 Redis 클라이언트를 등록한다.
// 용도별 분리(user_lock, user_cache, ranking 등)를 위해 여러 인스턴스를 등록할 수 있다.
func Init(name, host, port, password string, db int) error {
	c := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	if err := c.Ping(context.Background()).Err(); err != nil {
		_ = c.Close()
		return fmt.Errorf("failed to connect redis [%s]: %w", name, err)
	}

	mu.Lock()
	instances[name] = c
	mu.Unlock()
	return nil
}

// Get 이름으로 등록된 Redis 클라이언트를 반환한다.
func Get(name string) *redis.Client {
	mu.RLock()
	defer mu.RUnlock()
	return instances[name]
}

// CloseAll 등록된 모든 Redis 클라이언트를 닫는다. 서버 종료 시 호출.
func CloseAll() {
	mu.Lock()
	defer mu.Unlock()
	for name, c := range instances {
		_ = c.Close()
		delete(instances, name)
	}
}

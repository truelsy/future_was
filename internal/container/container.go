package container

import (
	"future_next_baseball/internal/cache"
	"future_next_baseball/internal/database"
)

// Container 핸들러 전체에서 공유되는 의존성을 보관한다.
type Container struct {
	GameDB    *database.Database
	UserCache *cache.UserCache
	UserLock  *cache.UserLock
}

func New() *Container {
	return &Container{
		GameDB:    database.GetShard(0),
		UserCache: cache.NewUserCache(),
		UserLock:  cache.NewUserLock(),
	}
}

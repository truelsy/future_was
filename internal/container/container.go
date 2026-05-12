package container

import (
	"future_was/internal/cache"
	"future_was/internal/database"
	"future_was/internal/design"
	"future_was/internal/resource"
)

// Container 핸들러 전체에서 공유되는 의존성을 보관한다.
type Container struct {
	GameDB         *database.Database
	UserCache      *cache.UserCache
	UserLock       *cache.UserLock
	ClubCache      *cache.ClubCache
	ClubLock       *cache.ClubLock
	DesignStore    *design.Store
	DesignSyncer   *design.Syncer
	ResourceStore  *resource.Store
	ResourceSyncer *resource.Syncer
}

// New 기본 의존성으로 Container를 생성한다.
// design 관련 의존성은 main에서 별도로 초기화 후 주입한다.
func New(designStore *design.Store, designSyncer *design.Syncer, resourceStore *resource.Store, resourceSyncer *resource.Syncer) *Container {
	return &Container{
		GameDB:         database.GetGameDB(),
		UserCache:      cache.NewUserCache(),
		UserLock:       cache.NewUserLock(),
		ClubCache:      cache.NewClubCache(),
		ClubLock:       cache.NewClubLock(),
		DesignStore:    designStore,
		DesignSyncer:   designSyncer,
		ResourceStore:  resourceStore,
		ResourceSyncer: resourceSyncer,
	}
}

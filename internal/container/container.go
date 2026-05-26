package container

import (
	"future_cpbl_web_server/config"
	"future_cpbl_web_server/internal/cache"
	"future_cpbl_web_server/internal/database"
	"future_cpbl_web_server/internal/design"
	"future_cpbl_web_server/internal/resource"
)

// Container 핸들러 전체에서 공유되는 의존성을 보관한다.
type Container struct {
	Stage          config.Stage // 환경 단계 (local / dev / qa / staging / live) — admin 노출 등 분기에 사용
	GameDB         *database.Database
	UserCache      *cache.UserCache
	UserLock       *cache.UserLock
	UserSession    *cache.UserSession
	ClubCache      *cache.ClubCache
	ClubLock       *cache.ClubLock
	DesignStore    *design.Store
	DesignSyncer   *design.Syncer
	ResourceStore  *resource.Store
	ResourceSyncer *resource.Syncer
}

// New 기본 의존성으로 Container를 생성한다.
// design 관련 의존성은 main에서 별도로 초기화 후 주입한다.
func New(stage config.Stage, designStore *design.Store, designSyncer *design.Syncer, resourceStore *resource.Store, resourceSyncer *resource.Syncer) *Container {
	return &Container{
		Stage:          stage,
		GameDB:         database.GetGameDB(),
		UserCache:      cache.NewUserCache(),
		UserLock:       cache.NewUserLock(),
		UserSession:    cache.NewUserSession(),
		ClubCache:      cache.NewClubCache(),
		ClubLock:       cache.NewClubLock(),
		DesignStore:    designStore,
		DesignSyncer:   designSyncer,
		ResourceStore:  resourceStore,
		ResourceSyncer: resourceSyncer,
	}
}

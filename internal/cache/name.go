package cache

// Redis 인스턴스 이름 상수. config.yaml의 redis 항목 name과 매칭된다.
const (
	NameLock         = "lock"
	NameUserCache    = "user_cache"
	NameClubCache    = "club_cache"
	NameReloadPubSub = "reload_pubsub"
)

package cache

// Redis 인스턴스 이름 상수. config.yaml의 redis 항목 name과 매칭된다.
const (
	NameUserLock   = "user_lock"
	NameUserCache  = "user_cache"
	NameDesignSync = "design_sync"
)

package uow

import "errors"

// Owner LoadOne/LoadList의 소유자 스코프.
// user는 1유저당 1캐시(user:{id}), club은 멤버 N명이 공유(club:{id}).
type Owner int

const (
	OwnerUser Owner = iota
	OwnerClub
)

// ErrNoClubID clubID가 설정되지 않은 상태에서 OwnerClub으로 로더를 호출했을 때 반환된다.
var ErrNoClubID = errors.New("uow: clubID is not set")

// ErrNoUserID userID가 설정되지 않은 상태에서 로더를 호출했을 때 반환된다.
var ErrNoUserID = errors.New("uow: userID is not set")

// entityCache UserCache/ClubCache가 공통으로 만족하는 내부 인터페이스.
// 외부에 노출하지 않으며, Loader가 user/club을 동일 코드로 다루기 위한 최소 helper이다.
type entityCache interface {
	Get(id uint64, field string, dest any) error
	Set(id uint64, field string, value any) error
	SetMulti(id uint64, fields map[string]any) error
}

// scope Owner에 해당하는 id/store/cache/where 정보 묶음.
// uow 내부 Loader 구현용 — 외부 노출 X.
type scope struct {
	id      uint64
	store   map[string]any
	cache   entityCache
	where   string
	errNoID error
}

// scopeOf Owner에 해당하는 scope를 반환한다.
func (u *UnitOfWork) scopeOf(o Owner) scope {
	switch o {
	case OwnerClub:
		return scope{
			id:      u.clubID,
			store:   u.clubStore,
			cache:   u.c.ClubCache,
			where:   "club_id = ?",
			errNoID: ErrNoClubID,
		}
	default: // OwnerUser
		return scope{
			id:      u.userID,
			store:   u.store,
			cache:   u.c.UserCache,
			where:   "user_id = ?",
			errNoID: ErrNoUserID,
		}
	}
}

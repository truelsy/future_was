package uow

import (
	"reflect"

	"future_next_baseball/internal/database"
)

// QueryClub 다른 클럽의 단일 엔티티를 조회한다 (ClubCache → DB → ClubCache).
//
// UoW의 clubStore를 건드리지 않으므로 한 요청에서 여러 clubID를 자유롭게 조회 가능
// (랭킹·검색·프로필 보기 등). 자기 클럽을 변경할 때는 SetClubID + LoadOne(OwnerClub)을 사용한다.
// club, err := uow.QueryClub[*model.Club](u, otherClubID, uow.FieldClubInfo, u.Container().GameDB)
func QueryClub[T database.Model](u *UnitOfWork, clubID uint64, field string, db *database.Database) (T, error) {
	var zero T
	if clubID == 0 {
		return zero, ErrNoClubID
	}

	dest := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)

	// 1. ClubCache hit
	if err := u.c.ClubCache.Get(clubID, field, dest); err == nil {
		return dest, nil
	}

	// 2. DB
	if err := db.FindOne(dest, "club_id = ?", clubID); err != nil {
		return zero, err
	}

	// 3. cache 채움
	_ = u.c.ClubCache.Set(clubID, field, dest)
	return dest, nil
}

// QueryClubList 다른 클럽들의 슬라이스를 조회한다 (ClubCache → DB → ClubCache).
// clubMembers, err := uow.QueryClubList[*model.ClubMember](u, otherClubID, uow.FieldClubMember, u.Container().GameDB)
func QueryClubList[T database.Model](u *UnitOfWork, clubID uint64, field string, db *database.Database) ([]T, error) {
	if clubID == 0 {
		return nil, ErrNoClubID
	}

	var list []T
	if err := u.c.ClubCache.Get(clubID, field, &list); err == nil {
		return list, nil
	}

	var zero T
	zeroVal := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
	if err := db.FindList(&list, zeroVal, "club_id = ?", nil, clubID); err != nil {
		return nil, err
	}
	_ = u.c.ClubCache.Set(clubID, field, list)
	return list, nil
}

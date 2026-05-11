// Package uow는 요청 단위의 Unit of Work를 제공한다.
// 데이터 읽기/쓰기를 축적한 뒤, 커밋 시 DB와 캐시에 일괄 반영한다.
//
// UnitOfWork는 고루틴 안전하지 않다. 요청당 하나의 인스턴스만 사용해야 한다.
package uow

import (
	"errors"
	"fmt"
	"reflect"

	"future_next_baseball/internal/container"
	"future_next_baseball/internal/database"
	"future_next_baseball/internal/design"
	"future_next_baseball/internal/model"

	"github.com/jmoiron/sqlx"
)

// ErrNoUserID userID가 설정되지 않은 상태에서 로더를 호출했을 때 반환된다.
var ErrNoUserID = errors.New("uow: userID is not set")

// dbOp는 특정 데이터베이스에 바인딩된 큐잉된 쓰기 작업이다.
type dbOp struct {
	db *database.Database
	fn func(tx *sqlx.Tx) error
}

// UnitOfWork 요청 내에서 읽기(지연 로딩)와 쓰기(ops 큐잉)를 축적한다.
// Commit 시 DB별 트랜잭션으로 쓰기를 실행하고, 성공 시 캐시를 갱신한다.
//
// scope 분리:
//   - user scope: userID + store + UserCache (1유저당 1캐시)
//   - club scope: clubID + clubStore + ClubCache (멤버 N명 공유)
//
// LoadOne/LoadList의 Owner 인자로 두 scope를 분기한다.
type UnitOfWork struct {
	c         *container.Container
	userID    uint64
	clubID    uint64
	catalog   *design.Catalog
	store     map[string]any // user scope
	clubStore map[string]any // club scope
	ops       []dbOp
}

// New 요청 단위 UnitOfWork를 생성한다. userID는 미확정 시 0 가능
// (예: 로그인 시). 확정 후 SetUserID를 호출한다.
// clubID는 클럽 액션 핸들러에서 SetClubID로 주입한다.
// catalog는 요청의 client_version으로 라우팅된 디자인 Catalog이다.
func New(c *container.Container, userID uint64, catalog *design.Catalog) *UnitOfWork {
	return &UnitOfWork{
		c:         c,
		userID:    userID,
		catalog:   catalog,
		store:     make(map[string]any),
		clubStore: make(map[string]any),
	}
}

func (u *UnitOfWork) UserID() uint64                  { return u.userID }
func (u *UnitOfWork) SetUserID(id uint64)             { u.userID = id }
func (u *UnitOfWork) ClubID() uint64                  { return u.clubID }
func (u *UnitOfWork) SetClubID(id uint64)             { u.clubID = id }
func (u *UnitOfWork) Container() *container.Container { return u.c }
func (u *UnitOfWork) Catalog() *design.Catalog        { return u.catalog }

// ShardDB 유저의 Account.DBShardID에 해당하는 Database를 반환한다.
// 계정이 로드되지 않았거나 shard가 등록되지 않은 경우 nil을 반환한다.
// 호출자가 nil 체크 후 사용해야 한다 (uow.Update 등은 nil DB 시 Commit에서 실패).
func (u *UnitOfWork) ShardDB() *database.Database {
	acc, err := u.Account()
	if err != nil || acc == nil {
		return nil
	}
	return database.GetShard(acc.DBShardID)
}

// ---------------------------------------------------------------------------
// 제네릭 로더
// ---------------------------------------------------------------------------

// LoadOne 단일 엔티티를 지연 로딩한다 (scope store → Redis → DB).
// owner로 user/club scope를 분기. T는 포인터 타입으로 database.Model을 구현해야 한다.
func LoadOne[T database.Model](u *UnitOfWork, owner Owner, field string, db *database.Database) (T, error) {
	s := u.scopeOf(owner)

	if v, ok := s.store[field]; ok {
		return v.(T), nil
	}
	var zero T
	if s.id == 0 {
		return zero, s.errNoID
	}

	dest := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
	if err := s.cache.Get(s.id, field, dest); err == nil {
		s.store[field] = dest
		return dest, nil
	}

	if err := db.FindOne(dest, s.where, s.id); err != nil {
		return zero, err
	}
	s.store[field] = dest
	_ = s.cache.Set(s.id, field, dest)
	return dest, nil
}

// LoadList 엔티티 슬라이스를 지연 로딩한다 (scope store → Redis → DB).
// owner로 user/club scope를 분기. store에 []T를 저장하므로, 요소를 수정하면
// scope store에 자동 반영된다.
func LoadList[T database.Model](u *UnitOfWork, owner Owner, field string, db *database.Database) ([]T, error) {
	s := u.scopeOf(owner)

	if v, ok := s.store[field]; ok {
		return v.([]T), nil
	}
	if s.id == 0 {
		return nil, s.errNoID
	}

	var list []T
	if err := s.cache.Get(s.id, field, &list); err == nil {
		s.store[field] = list
		return list, nil
	}

	var zero T
	zeroVal := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
	if err := db.FindList(&list, zeroVal, s.where, nil, s.id); err != nil {
		return nil, err
	}
	s.store[field] = list
	_ = s.cache.Set(s.id, field, list)
	return list, nil
}

// ---------------------------------------------------------------------------
// 편의 래퍼 (기존 호출부 시그니처 유지)
// ---------------------------------------------------------------------------

func (u *UnitOfWork) Account() (*model.Account, error) {
	return LoadOne[*model.Account](u, OwnerUser, FieldAccount, u.c.GameDB)
}
func (u *UnitOfWork) Assets() ([]*model.Asset, error) {
	acc, err := u.Account()
	if err != nil {
		return nil, err
	}
	shardDB := database.GetShard(acc.DBShardID)
	if shardDB == nil {
		return nil, fmt.Errorf("assets: shard_id=%d에 해당하는 DB를 찾을 수 없음", acc.DBShardID)
	}
	return LoadList[*model.Asset](u, OwnerUser, FieldAssets, shardDB)
}

// Cards Account.DBShardID로 결정된 shard DB에서 카드를 로딩한다.
func (u *UnitOfWork) Cards() ([]*model.Card, error) {
	acc, err := u.Account()
	if err != nil {
		return nil, fmt.Errorf("cards: shard 라우팅을 위한 계정 로드 실패: %w", err)
	}
	shardDB := database.GetShard(acc.DBShardID)
	if shardDB == nil {
		return nil, fmt.Errorf("cards: shard_id=%d에 해당하는 DB를 찾을 수 없음", acc.DBShardID)
	}
	return LoadList[*model.Card](u, OwnerUser, FieldCards, shardDB)
}

// ---------------------------------------------------------------------------
// 쓰기 작업
// ---------------------------------------------------------------------------

// queueOp DB 쓰기 작업을 큐잉한다. 서비스에서 도메인별 쓰기를 등록할 때 사용한다.
func (u *UnitOfWork) queueOp(db *database.Database, fn func(tx *sqlx.Tx) error) {
	u.ops = append(u.ops, dbOp{db: db, fn: fn})
}

// Create store에 추가하고 INSERT를 큐잉한다.
// Commit 시 트랜잭션 내에서 실행되며, auto-increment PK가 SetPrimaryKey로 반영된다.
// PK는 Commit 후에만 참조 가능하다. 즉시 PK가 필요하면 CreateNow를 사용한다.
func Create[T database.Model](u *UnitOfWork, field string, m T, db *database.Database) {
	storeModel(u, field, m)

	u.queueOp(db, func(tx *sqlx.Tx) error {
		id, err := database.CreateTx(tx, m)
		if err != nil {
			return err
		}
		m.SetPrimaryKey(id)
		return nil
	})
}

// CreateNow 즉시 INSERT하고 auto-increment PK를 SetPrimaryKey로 반영한다.
// 후속 로직에서 PK가 바로 필요한 경우 사용한다. Rollback 불가.
// FieldAccount로 호출 시 PK가 userID이므로 UoW의 userID도 자동 설정된다.
func CreateNow[T database.Model](u *UnitOfWork, field string, m T, db *database.Database) error {
	id, err := db.Create(m)
	if err != nil {
		return err
	}
	m.SetPrimaryKey(id)
	storeModel(u, field, m)

	if field == FieldAccount {
		u.SetUserID(uint64(id))
	}
	return nil
}

// storeModel Model의 IsSingleton 여부에 따라 store에 단일 또는 슬라이스로 저장한다.
func storeModel[T database.Model](u *UnitOfWork, field string, m T) {
	if m.IsSingleton() {
		u.store[field] = m
		return
	}
	if list, ok := u.store[field].([]T); ok {
		u.store[field] = append(list, m)
	} else {
		u.store[field] = []T{m}
	}
}

// Update 지정된 컬럼의 UPDATE를 큐잉한다. 컬럼 미지정 시 PK 제외 전체 컬럼을 업데이트한다.
func Update[T database.Model](u *UnitOfWork, m T, db *database.Database, columns ...string) {
	u.queueOp(db, func(tx *sqlx.Tx) error {
		_, err := database.SaveTx(tx, m, columns...)
		return err
	})
}

// ---------------------------------------------------------------------------
// 커밋
// ---------------------------------------------------------------------------

// Commit 은 큐잉된 모든 작업을 DB 트랜잭션으로 실행한다 (DB별 그룹핑).
// 성공 시 로딩된 데이터를 유저 캐시에 HSet으로 기록한다.
// 실패 시 에러를 반환하며, 호출자가 캐시 무효화를 담당한다
// (예: handler.CommitOrRollback).
func (u *UnitOfWork) Commit() error {
	// DB 인스턴스별로 ops를 그룹핑한다.
	if len(u.ops) > 0 {
		type group struct {
			db  *database.Database
			fns []func(tx *sqlx.Tx) error
		}
		var groups []group
		idx := make(map[*database.Database]int)
		for _, op := range u.ops {
			i, ok := idx[op.db]
			if !ok {
				i = len(groups)
				idx[op.db] = i
				groups = append(groups, group{db: op.db})
			}
			groups[i].fns = append(groups[i].fns, op.fn)
		}

		// 각 그룹을 개별 트랜잭션으로 실행한다.
		for _, g := range groups {
			err := g.db.Transaction(func(tx *sqlx.Tx) error {
				for _, fn := range g.fns {
					if err := fn(tx); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("uow commit: %w", err)
			}
		}

		u.ops = nil
	}

	// scope별 캐시 갱신.
	if u.userID != 0 && len(u.store) > 0 {
		if err := u.c.UserCache.SetMulti(u.userID, u.store); err != nil {
			return fmt.Errorf("user cache set: %w", err)
		}
	}
	if u.clubID != 0 && len(u.clubStore) > 0 {
		if err := u.c.ClubCache.SetMulti(u.clubID, u.clubStore); err != nil {
			return fmt.Errorf("club cache set: %w", err)
		}
	}
	return nil
}

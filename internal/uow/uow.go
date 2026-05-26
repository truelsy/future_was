// Package uow는 요청 단위의 Unit of Work를 제공한다.
// 데이터 읽기/쓰기를 축적한 뒤, 커밋 시 DB와 캐시에 일괄 반영한다.
//
// UnitOfWork는 고루틴 안전하지 않다. 요청당 하나의 인스턴스만 사용해야 한다.
package uow

import (
	"fmt"
	"reflect"

	"future_cpbl_web_server/internal/container"
	"future_cpbl_web_server/internal/database"
	"future_cpbl_web_server/internal/design"

	"github.com/jmoiron/sqlx"
)

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
// 모든 LoadOne/LoadList/Create/CreateNow/Update 는 EntityKind 인자를 받으며,
// EntityKind.Owner 로 두 scope 를 분기하고 EntityKind.IsGameDB 로 GameDB/ShardDB 를 자동 라우팅한다.
type UnitOfWork struct {
	c         *container.Container
	userID    uint64
	clubID    uint64
	catalog   *design.Catalog
	store     map[string]any // user scope
	clubStore map[string]any // club scope
	dbOps     []dbOp
	dirty     map[any]string // 같은 포인터가 여러 번 markDirty 돼도 1회만 sync 첨부.
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
		dirty:     make(map[any]string),
	}
}

// Dirty 이번 요청에서 변경된(생성/수정) 모델을 EntityKind.Name 별로 반환한다.
// dispatch가 commit 후 envelope의 sync 필드를 채울 때 사용.
func (u *UnitOfWork) Dirty() map[string][]any {
	if len(u.dirty) == 0 {
		return nil
	}
	out := make(map[string][]any, len(u.dirty))
	for m, name := range u.dirty {
		out[name] = append(out[name], m)
	}
	return out
}

// markDirty 모델 인스턴스를 dirty 추적에 추가한다.
// 등록되지 않은 모델 타입은 무시 (sync 대상 아님).
func (u *UnitOfWork) markDirty(m any) {
	name, ok := entityNameOf(m)
	if !ok {
		return
	}
	u.dirty[m] = name
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
// entity.Owner 로 user/club scope를 분기, entity.IsGameDB 로 GameDB/ShardDB 를 라우팅한다.
// T는 포인터 타입으로 database.Model을 구현해야 한다.
func LoadOne[T database.Model](u *UnitOfWork, entity EntityKind) (T, error) {
	s := u.scopeOf(entity.Owner)

	if v, ok := s.store[entity.Name]; ok {
		return v.(T), nil
	}
	var zero T
	if s.id == 0 {
		return zero, s.errNoID
	}

	dest := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
	if err := s.cache.Get(s.id, entity.Name, dest); err == nil {
		s.store[entity.Name] = dest
		return dest, nil
	}

	var db *database.Database
	if entity.IsGameDB {
		db = u.c.GameDB
	} else {
		db = u.ShardDB()
	}

	if err := db.FindOne(dest, s.where, s.id); err != nil {
		return zero, err
	}
	s.store[entity.Name] = dest
	_ = s.cache.Set(s.id, entity.Name, dest)
	return dest, nil
}

// LoadList 엔티티 슬라이스를 지연 로딩한다 (scope store → Redis → DB).
// entity.Owner 로 user/club scope를 분기, entity.IsGameDB 로 GameDB/ShardDB 를 라우팅한다.
// store에 []T를 저장하므로, 요소를 수정하면 scope store에 자동 반영된다.
func LoadList[T database.Model](u *UnitOfWork, entity EntityKind) ([]T, error) {
	s := u.scopeOf(entity.Owner)

	if v, ok := s.store[entity.Name]; ok {
		return v.([]T), nil
	}
	if s.id == 0 {
		return nil, s.errNoID
	}

	var list []T
	if err := s.cache.Get(s.id, entity.Name, &list); err == nil {
		s.store[entity.Name] = list
		return list, nil
	}

	var db *database.Database
	if entity.IsGameDB {
		db = u.c.GameDB
	} else {
		db = u.ShardDB()
	}

	var zero T
	zeroVal := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
	if err := db.FindList(&list, zeroVal, s.where, nil, s.id); err != nil {
		return nil, err
	}
	s.store[entity.Name] = list
	_ = s.cache.Set(s.id, entity.Name, list)
	return list, nil
}

// ---------------------------------------------------------------------------
// 쓰기 작업
// ---------------------------------------------------------------------------

// queueOp DB 쓰기 작업을 큐잉한다. 서비스에서 도메인별 쓰기를 등록할 때 사용한다.
func (u *UnitOfWork) queueOp(db *database.Database, fn func(tx *sqlx.Tx) error) {
	u.dbOps = append(u.dbOps, dbOp{db: db, fn: fn})
}

// Create store에 추가하고 INSERT를 큐잉한다.
// entity.Owner 로 user/club scope 를 분기, entity.IsGameDB 로 GameDB/ShardDB 를 라우팅한다.
// Commit 시 트랜잭션 내에서 실행되며, auto-increment PK 가 SetPrimaryKey 로 반영된다.
// PK는 Commit 후에만 참조 가능하다. 즉시 PK가 필요하면 CreateNow를 사용한다.
func Create[T database.Model](u *UnitOfWork, entity EntityKind, m T) {
	storeModel(u, entity, m)
	u.markDirty(m)

	var db *database.Database
	if entity.IsGameDB {
		db = u.c.GameDB
	} else {
		db = u.ShardDB()
	}

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
// entity.Owner 로 user/club scope 를 분기, entity.IsGameDB 로 GameDB/ShardDB 를 라우팅한다.
// 후속 로직에서 PK가 바로 필요한 경우 사용한다. Rollback 불가.
// EntityAccount로 호출 시 PK가 userID이므로 UoW의 userID도 자동 설정된다.
func CreateNow[T database.Model](u *UnitOfWork, entity EntityKind, m T) error {
	var db *database.Database
	if entity.IsGameDB {
		db = u.c.GameDB
	} else {
		db = u.ShardDB()
	}

	id, err := db.Create(m)
	if err != nil {
		return err
	}
	m.SetPrimaryKey(id)

	storeModel(u, entity, m)
	u.markDirty(m)

	if entity == EntityAccount {
		u.SetUserID(uint64(id))
	}
	return nil
}

// Update 지정된 컬럼의 UPDATE를 큐잉한다. entity.IsGameDB 로 GameDB/ShardDB 를 라우팅한다.
// store는 건드리지 않으며 markDirty 로 변경 추적만 한다 (캐시는 Commit 시 store 통해 갱신).
// 컬럼 미지정 시 PK 제외 전체 컬럼을 업데이트한다.
func Update[T database.Model](u *UnitOfWork, entity EntityKind, m T, columns ...string) {
	u.markDirty(m)

	var db *database.Database
	if entity.IsGameDB {
		db = u.c.GameDB
	} else {
		db = u.ShardDB()
	}

	u.queueOp(db, func(tx *sqlx.Tx) error {
		_, err := database.SaveTx(tx, m, columns...)
		return err
	})
}

// storeModel Model의 IsSingleton 여부에 따라 entity.Owner scope 의 store 에 단일 또는 슬라이스로 저장한다.
func storeModel[T database.Model](u *UnitOfWork, entity EntityKind, m T) {
	s := u.scopeOf(entity.Owner)
	if m.IsSingleton() {
		s.store[entity.Name] = m
		return
	}
	if list, ok := s.store[entity.Name].([]T); ok {
		s.store[entity.Name] = append(list, m)
	} else {
		s.store[entity.Name] = []T{m}
	}
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
	if len(u.dbOps) > 0 {
		type group struct {
			db  *database.Database
			fns []func(tx *sqlx.Tx) error
		}
		var groups []group
		idx := make(map[*database.Database]int)
		for _, op := range u.dbOps {
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

		u.dbOps = nil
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

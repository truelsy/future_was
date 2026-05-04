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
type UnitOfWork struct {
	c      *container.Container
	userID uint64
	store  map[string]any // 필드명 → 로딩된 데이터 (*T 또는 []T)
	ops    []dbOp
}

// New 요청 단위 UnitOfWork를 생성한다. userID는 미확정 시 0 가능
// (예: 로그인 시). 확정 후 SetUserID를 호출한다.
func New(c *container.Container, userID uint64) *UnitOfWork {
	return &UnitOfWork{c: c, userID: userID, store: make(map[string]any)}
}

func (u *UnitOfWork) UserID() uint64                  { return u.userID }
func (u *UnitOfWork) SetUserID(id uint64)             { u.userID = id }
func (u *UnitOfWork) Container() *container.Container { return u.c }

// ShardDB 유저의 Shard정보를 가져온다.
func (u *UnitOfWork) ShardDB() *database.Database {
	acc, _ := u.Account()
	return database.GetShard(acc.DBShardID)
}

// ---------------------------------------------------------------------------
// 제네릭 로더
// ---------------------------------------------------------------------------

// LoadOne 단일 엔티티를 지연 로딩한다 (UoW store → Redis → DB).
// T는 포인터 타입으로 database.Model을 구현해야 한다 (예: *model.Account).
func LoadOne[T database.Model](u *UnitOfWork, field string, db *database.Database) (T, error) {
	if v, ok := u.store[field]; ok {
		return v.(T), nil
	}
	var zero T
	if u.userID == 0 {
		return zero, ErrNoUserID
	}

	dest := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
	if err := u.c.UserCache.Get(u.userID, field, dest); err == nil {
		u.store[field] = dest
		return dest, nil
	}

	if err := db.FindOne(dest, "user_id = ?", u.userID); err != nil {
		return zero, err
	}
	u.store[field] = dest
	_ = u.c.UserCache.Set(u.userID, field, dest)
	return dest, nil
}

// LoadList 엔티티 슬라이스를 지연 로딩한다 (UoW store → Redis → DB).
// T는 포인터 타입으로 database.Model을 구현해야 한다 (예: *model.Asset).
// store에 []T를 저장하므로, 요소를 수정하면 store 스냅샷에 자동 반영된다.
func LoadList[T database.Model](u *UnitOfWork, field string, db *database.Database) ([]T, error) {
	if v, ok := u.store[field]; ok {
		return v.([]T), nil
	}
	if u.userID == 0 {
		return nil, ErrNoUserID
	}

	var list []T
	if err := u.c.UserCache.Get(u.userID, field, &list); err == nil {
		u.store[field] = list
		return list, nil
	}

	var zero T
	zeroVal := reflect.New(reflect.TypeOf(zero).Elem()).Interface().(T)
	if err := db.FindList(&list, zeroVal, "user_id = ?", nil, u.userID); err != nil {
		return nil, err
	}
	u.store[field] = list
	_ = u.c.UserCache.Set(u.userID, field, list)
	return list, nil
}

// ---------------------------------------------------------------------------
// 편의 래퍼 (기존 호출부 시그니처 유지)
// ---------------------------------------------------------------------------

func (u *UnitOfWork) Account() (*model.Account, error) {
	return LoadOne[*model.Account](u, FieldAccount, u.c.GameDB)
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
	return LoadList[*model.Asset](u, FieldAssets, shardDB)
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
	return LoadList[*model.Card](u, FieldCards, shardDB)
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
// 성공 시 로딩된 스냅샷을 유저 캐시에 HSet으로 기록한다.
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

	// ops가 없어도 store가 있으면 캐시에 반영한다.
	if u.userID == 0 || len(u.store) == 0 {
		return nil
	}

	return u.c.UserCache.SetMulti(u.userID, u.store)
}

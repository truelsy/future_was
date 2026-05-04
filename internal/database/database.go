package database

import (
	"fmt"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// Database 는 *sqlx.DB 인스턴스를 래핑하여 편의 메서드를 제공한다.
type Database struct {
	db *sqlx.DB
}

var (
	instances = make(map[string]*Database)
	shardMap  = make(map[int8]*Database)
	mu        sync.RWMutex
)

// Init 은 새 DB 연결을 생성하고 지정된 이름으로 등록한다.
func Init(name, dsn string) (*Database, error) {
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database [%s]: %w", name, err)
	}

	d := &Database{db: db}

	mu.Lock()
	instances[name] = d
	mu.Unlock()

	return d, nil
}

// Get 은 이름으로 등록된 Database 를 반환한다.
func Get(name string) *Database {
	mu.RLock()
	defer mu.RUnlock()
	return instances[name]
}

// RegisterShard 는 shard ID를 Database 인스턴스에 매핑한다.
func RegisterShard(shardID int8, db *Database) {
	mu.Lock()
	shardMap[shardID] = db
	mu.Unlock()
}

// GetShard 지정된 shard ID에 해당하는 Database를 반환한다.
func GetShard(shardID int8) *Database {
	mu.RLock()
	defer mu.RUnlock()
	return shardMap[shardID]
}

// CloseAll 등록된 모든 Database 연결을 닫는다. 서버 종료 시 호출.
func CloseAll() {
	mu.Lock()
	defer mu.Unlock()
	for name, d := range instances {
		_ = d.db.Close()
		delete(instances, name)
	}
	for id := range shardMap {
		delete(shardMap, id)
	}
}

// SqlxDB 내부 *sqlx.DB 인스턴스를 반환한다.
func (d *Database) SqlxDB() *sqlx.DB {
	return d.db
}

// QueryOption 목록 조회의 선택적 파라미터를 담는다.
type QueryOption struct {
	OrderBy string
	Limit   int
	Offset  int
}

// FindOne 단일 레코드를 조회한다. Model 메타데이터로 SELECT를 생성한다.
// 예: db.FindOne(&account, "channel_uid = ? AND is_active > 0", 123)
func (d *Database) FindOne(dest Model, where string, args ...any) error {
	meta := getMeta(dest)
	query := buildSelect(meta, where)
	return d.db.Get(dest, query, args...)
}

// FindList 여러 레코드를 조회한다. dest는 Model 슬라이스의 포인터여야 한다.
// model은 테이블 메타데이터 추출용 제로값 인스턴스이다.
// 예: db.FindList(&cards, model.Card{}, "user_id = ?", opt, 123)
func (d *Database) FindList(dest any, model Model, where string, opt *QueryOption, args ...any) error {
	meta := getMeta(model)
	query := buildSelect(meta, where)
	if opt != nil {
		if opt.OrderBy != "" {
			query += " ORDER BY " + opt.OrderBy
		}
		if opt.Limit > 0 {
			query += fmt.Sprintf(" LIMIT %d", opt.Limit)
		}
		if opt.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", opt.Offset)
		}
	}
	return d.db.Select(dest, query, args...)
}

// Create 새 레코드를 삽입한다. 컬럼은 Model 메타데이터에서 추출 (PK 제외).
// LastInsertId를 반환한다.
func (d *Database) Create(model Model) (int64, error) {
	meta := getMeta(model)
	query := buildInsert(meta)
	result, err := d.db.NamedExec(query, model)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// Save 레코드의 특정 컬럼을 업데이트한다. WHERE는 PK로 자동 생성된다.
// 컬럼을 지정하지 않으면 PK 제외 전체 컬럼을 업데이트한다.
// 예: db.Save(&account, "device_id", "update_time")
// 예: db.Save(&account) // PK 제외 전체 컬럼
func (d *Database) Save(model Model, columns ...string) (int64, error) {
	meta := getMeta(model)
	if len(columns) == 0 {
		columns = meta.insertCols
	}
	query := buildUpdate(meta, columns)
	result, err := d.db.NamedExec(query, model)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Remove PK로 레코드를 삭제한다.
func (d *Database) Remove(model Model, pkValue any) (int64, error) {
	meta := getMeta(model)
	query := buildDelete(meta)
	result, err := d.db.Exec(query, pkValue)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CountOf where 조건에 해당하는 건수를 반환한다.
// 예: db.CountOf(model.Account{}, "is_active > 0")
func (d *Database) CountOf(model Model, where string, args ...any) (int64, error) {
	meta := getMeta(model)
	query := buildCount(meta, where)
	var count int64
	err := d.db.Get(&count, query, args...)
	return count, err
}

// Transaction 주어진 함수를 DB 트랜잭션 내에서 실행한다.
func (d *Database) Transaction(fn func(tx *sqlx.Tx) error) error {
	tx, err := d.db.Beginx()
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// CreateTx 기존 트랜잭션 내에서 새 레코드를 삽입한다.
// LastInsertId를 반환한다.
func CreateTx(tx *sqlx.Tx, m Model) (int64, error) {
	meta := getMeta(m)
	query := buildInsert(meta)
	res, err := tx.NamedExec(query, m)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SaveTx 기존 트랜잭션 내에서 레코드의 특정 컬럼을 업데이트한다.
// WHERE는 PK로 자동 생성된다. 컬럼 미지정 시 PK 제외 전체 컬럼을 업데이트한다.
func SaveTx(tx *sqlx.Tx, m Model, columns ...string) (int64, error) {
	meta := getMeta(m)
	if len(columns) == 0 {
		columns = meta.insertCols
	}
	res, err := tx.NamedExec(buildUpdate(meta, columns), m)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RawGet 원시 SELECT 쿼리로 단일 행을 조회한다.
func (d *Database) RawGet(dest any, query string, args ...any) error {
	return d.db.Get(dest, query, args...)
}

// RawSelect 원시 SELECT 쿼리로 여러 행을 조회한다.
func (d *Database) RawSelect(dest any, query string, args ...any) error {
	return d.db.Select(dest, query, args...)
}

// RawExec 원시 쿼리(INSERT/UPDATE/DELETE)를 실행한다.
func (d *Database) RawExec(query string, args ...any) (int64, error) {
	result, err := d.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

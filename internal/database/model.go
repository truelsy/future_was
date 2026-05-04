package database

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Model 모든 DB 모델이 구현해야 하는 인터페이스이다.
type Model interface {
	TableName() string
	PrimaryKey() string
	SetPrimaryKey(id int64)
	// IsSingleton 단일 엔티티(Account 등)면 true, 슬라이스 엔티티(Asset, Card)면 false.
	// UoW store 저장 형태를 결정한다.
	IsSingleton() bool
}

// modelMeta는 모델 구조체의 파싱된 메타데이터를 보관한다.
type modelMeta struct {
	table      string
	pk         string
	columns    []string // PK 포함 전체 컬럼
	insertCols []string // PK 제외 컬럼 (auto-increment INSERT용)
}

var (
	metaCache   = make(map[reflect.Type]*modelMeta)
	metaCacheMu sync.RWMutex
)

// getMeta는 리플렉션을 사용하여 모델 메타데이터를 추출하고 캐싱한다.
func getMeta(m Model) *modelMeta {
	t := reflect.TypeOf(m)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	metaCacheMu.RLock()
	if meta, ok := metaCache[t]; ok {
		metaCacheMu.RUnlock()
		return meta
	}
	metaCacheMu.RUnlock()

	meta := &modelMeta{
		table: m.TableName(),
		pk:    m.PrimaryKey(),
	}

	for i := range t.NumField() {
		tag := t.Field(i).Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		meta.columns = append(meta.columns, tag)
		if tag != meta.pk {
			meta.insertCols = append(meta.insertCols, tag)
		}
	}

	metaCacheMu.Lock()
	metaCache[t] = meta
	metaCacheMu.Unlock()

	return meta
}

func buildSelect(meta *modelMeta, where string) string {
	return fmt.Sprintf("SELECT * FROM %s WHERE %s", meta.table, where)
}

func buildInsert(meta *modelMeta) string {
	named := make([]string, len(meta.insertCols))
	for i, col := range meta.insertCols {
		named[i] = ":" + col
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		meta.table,
		strings.Join(meta.insertCols, ", "),
		strings.Join(named, ", "),
	)
}

func buildUpdate(meta *modelMeta, columns []string) string {
	sets := make([]string, len(columns))
	for i, col := range columns {
		sets[i] = col + " = :" + col
	}
	return fmt.Sprintf("UPDATE %s SET %s WHERE %s = :%s",
		meta.table,
		strings.Join(sets, ", "),
		meta.pk, meta.pk,
	)
}

func buildDelete(meta *modelMeta) string {
	return fmt.Sprintf("DELETE FROM %s WHERE %s = ?", meta.table, meta.pk)
}

func buildCount(meta *modelMeta, where string) string {
	return fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", meta.table, where)
}

package uow

import (
	"reflect"
	"sync"

	"future_was/internal/database"
)

// modelFieldRegistry 모델 타입 → store field 매핑.
// 각 도메인의 setup에서 RegisterModelField로 1회 등록한 뒤, Create/CreateNow/Update
// 호출 시 자동으로 dirty 마킹에 사용된다. 미등록 모델은 sync에서 제외.
var (
	modelFieldRegistry   = map[reflect.Type]string{}
	modelFieldRegistryMu sync.RWMutex
)

// RegisterModelField 모델 타입 T를 store field에 매핑한다.
// T는 포인터 타입(예: *model.Item)이어야 한다.
//
// 사용 예 (setupItemHandler):
//
//	uow.RegisterModelField[*model.Item](uow.FieldItems)
func RegisterModelField[T database.Model](field string) {
	var zero T
	modelFieldRegistryMu.Lock()
	modelFieldRegistry[reflect.TypeOf(zero)] = field
	modelFieldRegistryMu.Unlock()
}

// fieldOf 모델 인스턴스의 등록된 field를 반환한다. 미등록 시 ("", false).
func fieldOf(m any) (string, bool) {
	modelFieldRegistryMu.RLock()
	defer modelFieldRegistryMu.RUnlock()
	field, ok := modelFieldRegistry[reflect.TypeOf(m)]
	return field, ok
}

package uow

import (
	"reflect"
	"sync"

	"future_cpbl_web_server/internal/database"
)

// modelRegistry 모델 타입 → EntityKind.Name 매핑.
// 각 도메인의 setup에서 RegisterModelEntity로 1회 등록한 뒤, Create/CreateNow/Update
// 호출 시 자동으로 dirty 마킹에 사용된다. 미등록 모델은 sync에서 제외.
var (
	modelRegistry   = map[reflect.Type]string{}
	modelRegistryMu sync.RWMutex
)

// RegisterModelEntity 모델 타입 T를 EntityKind 에 매핑한다.
// T는 포인터 타입(예: *model.Item)이어야 한다.
//
// 사용 예 (setupItemHandler):
//
//	uow.RegisterModelEntity[*model.Item](uow.EntityItems)
func RegisterModelEntity[T database.Model](entity EntityKind) {
	var zero T
	modelRegistryMu.Lock()
	modelRegistry[reflect.TypeOf(zero)] = entity.Name
	modelRegistryMu.Unlock()
}

// entityNameOf 모델 인스턴스의 등록된 EntityKind.Name 을 반환한다. 미등록 시 ("", false).
func entityNameOf(m any) (string, bool) {
	modelRegistryMu.RLock()
	defer modelRegistryMu.RUnlock()
	name, ok := modelRegistry[reflect.TypeOf(m)]
	return name, ok
}

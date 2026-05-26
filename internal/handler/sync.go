package handler

import (
	"future_cpbl_web_server/internal/uow"
	"sync"

	"future_cpbl_web_server/pb"
)

// SyncBuilder dirty 모델 슬라이스를 받아 *pb.SyncData 의 도메인 필드를 채운다.
// 각 도메인 setup에서 RegisterSyncBuilder로 등록한다.
type SyncBuilder func(dirty []any, dst *pb.SyncData)

var (
	syncBuilders   = map[string]SyncBuilder{}
	syncBuildersMu sync.RWMutex
)

// RegisterSyncBuilder EntityKind (uow.EntityItems 등) 에 대응하는 SyncData 빌더를 등록한다.
// 도메인 setup 단계에서 1회 호출. (도메인 추가 시 한 블록)
//
// 사용 예 (setupItemHandler):
//
//	handler.RegisterSyncBuilder(uow.EntityItems, func(dirty []any, dst *pb.SyncData) {
//	    for _, m := range dirty {
//	        if it, ok := m.(*model.Item); ok {
//	            dst.Items = append(dst.Items, toItemData(it))
//	        }
//	    }
//	})
func RegisterSyncBuilder(entity uow.EntityKind, fn SyncBuilder) {
	syncBuildersMu.Lock()
	syncBuilders[entity.Name] = fn
	syncBuildersMu.Unlock()
}

// BuildSyncData UoW의 dirty 모델들을 등록된 빌더로 *pb.SyncData 에 묶는다.
// dirty가 비어있거나 등록된 빌더와 매칭되는 게 없으면 nil 반환 (envelope에 빈 sync 첨부 회피).
func BuildSyncData(dirty map[string][]any) *pb.SyncData {
	if len(dirty) == 0 {
		return nil
	}
	syncBuildersMu.RLock()
	defer syncBuildersMu.RUnlock()

	out := &pb.SyncData{}
	nonEmpty := false
	for name, list := range dirty {
		if len(list) == 0 {
			continue
		}
		if fn, ok := syncBuilders[name]; ok {
			fn(list, out)
			nonEmpty = true
		}
	}
	if !nonEmpty {
		return nil
	}
	return out
}

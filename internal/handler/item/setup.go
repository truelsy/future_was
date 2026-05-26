package item

import (
	"future_cpbl_web_server/internal/container"
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/internal/model"
	"future_cpbl_web_server/internal/service"
	"future_cpbl_web_server/internal/uow"
	"future_cpbl_web_server/pb"

	"google.golang.org/protobuf/proto"
)

func init() {
	handler.RegisterSetup(setupItemHandler)
}

func setupItemHandler(c *container.Container) {
	svc := service.NewItemService()
	h := &itemHandler{svc: svc, c: c}

	handler.RegisterAction(handler.ActionGetItems, h.GetItems, func() proto.Message { return &pb.GetItemsRequest{} })

	// === 자동 동기화: 변경된 *model.Item 을 envelope sync.items 로 자동 첨부 ===
	uow.RegisterModelEntity[*model.Item](uow.EntityItems)
	handler.RegisterSyncBuilder(uow.EntityItems, func(dirty []any, dst *pb.SyncData) {
		for _, m := range dirty {
			if it, ok := m.(*model.Item); ok {
				dst.Items = append(dst.Items, toItemData(it))
			}
		}
	})
}

type itemHandler struct {
	svc *service.ItemService
	c   *container.Container
}

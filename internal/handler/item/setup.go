package item

import (
	"future_was/internal/container"
	"future_was/internal/handler"
	"future_was/internal/model"
	"future_was/internal/service"
	"future_was/internal/uow"
	"future_was/pb"

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
	uow.RegisterModelField[*model.Item](uow.FieldItems)
	handler.RegisterSyncBuilder(uow.FieldItems, func(dirty []any, dst *pb.SyncData) {
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

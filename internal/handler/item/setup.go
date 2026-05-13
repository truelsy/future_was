package item

import (
	"future_was/internal/container"
	"future_was/internal/handler"
	"future_was/internal/service"
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
}

type itemHandler struct {
	svc *service.ItemService
	c   *container.Container
}

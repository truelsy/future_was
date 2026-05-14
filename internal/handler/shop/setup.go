package shop

import (
	"future_was/internal/container"
	"future_was/internal/handler"
	"future_was/pb"

	"google.golang.org/protobuf/proto"
)

func init() {
	handler.RegisterSetup(setupShopHandler)
}

func setupShopHandler(c *container.Container) {
	h := &shopHandler{c: c}

	handler.RegisterAction(handler.ActionGetShopList, h.GetShopList, func() proto.Message { return &pb.GetShopListRequest{} })
}

type shopHandler struct {
	c *container.Container
}

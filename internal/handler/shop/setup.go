package shop

import (
	"future_cpbl_web_server/internal/container"
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/pb"

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

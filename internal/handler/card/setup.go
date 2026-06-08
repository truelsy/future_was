package card

import (
	"future_was/internal/container"
	"future_was/internal/handler"
	"future_was/internal/service"
	"future_was/pb"

	"google.golang.org/protobuf/proto"
)

func init() {
	handler.RegisterSetup(setupCardHandler)
}

func setupCardHandler(c *container.Container) {
	svc := service.NewCardService()
	h := &cardHandler{svc: svc, c: c}

	handler.RegisterAction(handler.ActionGetCards, h.GetCards, func() proto.Message { return &pb.GetCardsRequest{} })
	handler.RegisterAction(handler.ActionUpgradeCardLevel, h.UpgradeCardLevel, func() proto.Message { return &pb.UpgradeCardLevelRequest{} })
}

type cardHandler struct {
	svc *service.CardService
	c   *container.Container
}


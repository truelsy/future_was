package card

import (
	"future_next_baseball/internal/container"
	"future_next_baseball/internal/handler"
	"future_next_baseball/internal/service"
	"future_next_baseball/pb"

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


package card

import (
	"future_next_baseball/internal/handler"
	"future_next_baseball/internal/uow"
	"future_next_baseball/pb"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

func (h *cardHandler) UpgradeCardLevel(c echo.Context, body []byte) (proto.Message, error) {
	var req pb.UpgradeCardLevelRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		return nil, handler.BadRequest("invalid request")
	}

	u := handler.UoW(c)
	card, err := h.svc.GetCard(u, req.CardIdx)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, handler.Errorf(handler.CodeCardNotFound, "not found card. idx(%v)", req.CardIdx)
	}

	// 카드 레벨업 (임시 코드)
	card.Level += 1
	uow.Update(u, card, u.ShardDB())

	return &pb.UpgradeCardLevelResponse{
		Card: toCardData(card),
	}, nil
}

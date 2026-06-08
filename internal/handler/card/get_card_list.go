package card

import (
	"future_was/internal/handler"
	"future_was/pb"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

func (h *cardHandler) GetCards(c echo.Context, _ []byte) (proto.Message, error) {
	u := handler.UoW(c)
	cards, err := h.svc.GetCards(u)
	if err != nil {
		return nil, err
	}

	pbCards := make([]*pb.CardData, len(cards))
	for i, card := range cards {
		pbCards[i] = toCardData(card)
	}

	return &pb.GetCardsResponse{Cards: pbCards}, nil
}

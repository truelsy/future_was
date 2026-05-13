package item

import (
	"future_was/internal/handler"
	"future_was/internal/model"
	"future_was/pb"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

func (h *itemHandler) GetItems(c echo.Context, _ []byte) (proto.Message, error) {
	u := handler.UoW(c)
	items, err := h.svc.GetItems(u)
	if err != nil {
		return nil, err
	}

	pbItems := make([]*pb.ItemData, len(items))
	for i, item := range items {
		pbItems[i] = toItemData(item)
	}

	return &pb.GetItemsResponse{Items: pbItems}, nil
}

// toItemData converts Item to ItemData.
func toItemData(item *model.Item) *pb.ItemData {
	return &pb.ItemData{
		Idx:      item.Idx,
		ItemType: uint32(item.ItemType),
		ItemId:   item.ItemID,
		Amount:   item.Amount,
	}
}

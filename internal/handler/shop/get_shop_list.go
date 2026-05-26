package shop

import (
	"future_cpbl_web_server/internal/clock"
	"future_cpbl_web_server/internal/design/schema"
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/pb"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

func (h *shopHandler) GetShopList(c echo.Context, _ []byte) (proto.Message, error) {
	u := handler.UoW(c)

	pbShopList := make([]*pb.ShopData, 0)
	now := clock.Now().Unix()
	for _, v := range u.Catalog().Shop().All() {
		if v.UseFlag == 0 {
			continue
		}

		// 판매중인 상품
		if v.OpenTime <= now && now < v.CloseTime {
			pbShopList = append(pbShopList, toShopData(v))
		}
	}

	return &pb.GetShopListResponse{ShopList: pbShopList}, nil
}

// toShopData converts ShopListDesign to ShopData.
func toShopData(shop *schema.ShopListDesign) *pb.ShopData {
	return &pb.ShopData{
		Idx:       uint32(shop.Idx),
		ShopId:    uint32(shop.ShopId),
		Type:      uint32(shop.Type),
		OpenTime:  shop.OpenTime,
		CloseTime: shop.CloseTime,
		AssetId:   uint32(shop.CurrencyId),
		ShowFlag:  uint32(shop.ShowFlag),
		Price:     uint32(shop.Price),
		ShopType:  uint32(shop.ShopType),
		Ticker:    uint32(shop.Ticker),
	}
}

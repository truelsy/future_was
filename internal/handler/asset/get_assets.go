package asset

import (
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/internal/model"
	"future_cpbl_web_server/pb"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

func (h *assetHandler) GetAssets(c echo.Context, _ []byte) (proto.Message, error) {
	u := handler.UoW(c)
	assets, err := h.svc.GetAssets(u)
	if err != nil {
		return nil, err
	}

	pbAssets := make([]*pb.AssetData, len(assets))
	for i, asset := range assets {
		pbAssets[i] = toAssetData(asset)
	}

	return &pb.GetAssetsResponse{Assets: pbAssets}, nil
}

// toAssetData converts Asset to AssetData.
func toAssetData(asset *model.Asset) *pb.AssetData {
	return &pb.AssetData{
		Idx:              asset.Idx,
		AssetId:          asset.AssetID,
		Amount:           asset.Quantity,
		LastRechargeTime: asset.LastRechargeTime,
	}
}

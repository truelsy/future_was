package asset

import (
	"future_cpbl_web_server/internal/container"
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/internal/model"
	"future_cpbl_web_server/internal/service"
	"future_cpbl_web_server/internal/uow"
	"future_cpbl_web_server/pb"

	"google.golang.org/protobuf/proto"
)

func init() {
	handler.RegisterSetup(setupAssetHandler)
}

func setupAssetHandler(c *container.Container) {
	svc := service.NewAssetService()
	h := &assetHandler{svc: svc, c: c}

	handler.RegisterAction(handler.ActionGetAssets, h.GetAssets, func() proto.Message { return &pb.GetAssetsRequest{} })

	// === 자동 동기화: 변경된 *model.Item 을 envelope sync.assets 로 자동 첨부 ===
	uow.RegisterModelEntity[*model.Asset](uow.EntityAssets)
	handler.RegisterSyncBuilder(uow.EntityAssets, func(dirty []any, dst *pb.SyncData) {
		for _, m := range dirty {
			if asset, ok := m.(*model.Asset); ok {
				dst.Assets = append(dst.Assets, toAssetData(asset))
			}
		}
	})
}

type assetHandler struct {
	svc *service.AssetService
	c   *container.Container
}

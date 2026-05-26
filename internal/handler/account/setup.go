package account

import (
	"future_cpbl_web_server/internal/container"
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/internal/repository"
	"future_cpbl_web_server/internal/service"
	"future_cpbl_web_server/pb"

	"google.golang.org/protobuf/proto"
)

func init() {
	handler.RegisterSetup(setupAccountHandler)
}

func setupAccountHandler(c *container.Container) {
	repo := repository.NewAccountRepository(c.GameDB)
	h := &accountHandler{
		svc:      service.NewAccountService(repo),
		assetSvc: service.NewAssetService(),
		itemSvc:  service.NewItemService(),
		c:        c,
	}

	handler.RegisterNoAuthAction(handler.ActionLogin, h.Login, func() proto.Message { return &pb.LoginRequest{} })
}

type accountHandler struct {
	svc      *service.AccountService
	assetSvc *service.AssetService
	itemSvc  *service.ItemService
	c        *container.Container
}

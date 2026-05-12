package account

import (
	"future_was/internal/container"
	"future_was/internal/handler"
	"future_was/internal/repository"
	"future_was/internal/service"
	"future_was/pb"

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
		c:        c,
	}

	handler.RegisterAction(handler.ActionLogin, h.Login, func() proto.Message { return &pb.LoginRequest{} })
}

type accountHandler struct {
	svc      *service.AccountService
	assetSvc *service.AssetService
	c        *container.Container
}

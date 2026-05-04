package account

import (
	"future_next_baseball/internal/container"
	"future_next_baseball/internal/handler"
	"future_next_baseball/internal/repository"
	"future_next_baseball/internal/service"
	"future_next_baseball/pb"

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

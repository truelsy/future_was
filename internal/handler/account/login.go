package account

import (
	"future_was/internal/errcode"
	"future_was/internal/handler"
	"future_was/internal/log"
	"future_was/pb"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

func (h *accountHandler) Login(c echo.Context, body []byte) (proto.Message, error) {
	var req pb.LoginRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		return nil, errcode.BadRequest("invalid request")
	}

	if req.ChannelUid == 0 || req.DeviceId == "" {
		return nil, errcode.BadRequest("channel_uid and device_id are required")
	}

	u := handler.UoW(c)
	account, isNew, err := h.svc.Login(u, req.ChannelUid, req.DeviceId)
	if err != nil {
		return nil, err
	}

	// 재화 지급
	// todo: 임시코드
	asset := u.Catalog().Currency().Get(10000)
	if err := h.assetSvc.AddAsset(u, uint32(asset.ItemId), 100); err != nil {
		return nil, err
	}
	_ = h.assetSvc.ConsumeAsset(u, uint32(asset.ItemId), 30)

	// 카드 한장 조회
	// todo: 임시 코드
	batter := u.Catalog().BatData().Get(100739)
	log.Info().Interface("batter", batter).Msg("batter")

	return &pb.LoginResponse{
		UserId:     account.UserID,
		ChannelUid: account.ChannelUID,
		IsNew:      isNew,
	}, nil
}

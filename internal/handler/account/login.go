package account

import (
	"future_next_baseball/internal/handler"
	"future_next_baseball/pb"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

func (h *accountHandler) Login(c echo.Context, body []byte) (proto.Message, error) {
	var req pb.LoginRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		return nil, handler.BadRequest("invalid request")
	}

	if req.ChannelUid == 0 || req.DeviceId == "" {
		return nil, handler.BadRequest("channel_uid and device_id are required")
	}

	u := handler.UoW(c)
	account, isNew, err := h.svc.Login(u, req.ChannelUid, req.DeviceId)
	if err != nil {
		return nil, err
	}

	// 재화 지급
	// todo: 임시코드
	if err := h.assetSvc.AddAsset(u, 10000, 100); err != nil {
		return nil, err
	}

	h.assetSvc.ConsumeAsset(u, 10000, 30)

	return &pb.LoginResponse{
		UserId:     account.UserID,
		ChannelUid: account.ChannelUID,
		IsNew:      isNew,
	}, nil
}

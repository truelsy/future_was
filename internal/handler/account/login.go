package account

import (
	"future_cpbl_web_server/internal/errcode"
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/internal/log"
	"future_cpbl_web_server/pb"

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

	// 초기 재화 지급
	// todo: 임시코드
	if isNew {
		if err := h.assetSvc.AddAsset(u, 10000, 100); err != nil {
			return nil, err
		}

		// 초기 아이템 지급
		if err := h.itemSvc.AddItem(u, 25000, 5); err != nil {
			return nil, err
		}
		_ = h.itemSvc.ConsumeItem(u, 25000, 3)

		// 카드 한장 조회
		batter := u.Catalog().BatData().Get(100739)
		log.Info().Interface("batter", batter).Msg("batter")
	}

	token, err := h.c.UserSession.Set(account.UserID)
	if err != nil {
		return nil, errcode.Newf(errcode.CodeInternalError, "session set: %v", err)
	}

	return &pb.LoginResponse{
		UserId:       account.UserID,
		ChannelUid:   account.ChannelUID,
		IsNew:        isNew,
		SessionToken: token,
	}, nil
}

package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"future_next_baseball/internal/container"
	"future_next_baseball/internal/log"
	"future_next_baseball/internal/uow"
	"future_next_baseball/pb"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// SetupFunc 핸들러의 서비스를 초기화하고 액션을 등록하는 함수이다.
type SetupFunc func(c *container.Container)

var setupRegistry []SetupFunc
var ctn *container.Container

// RegisterSetup 핸들러의 셋업 함수를 레지스트리에 추가한다.
func RegisterSetup(fn SetupFunc) {
	setupRegistry = append(setupRegistry, fn)
}

// SetupAll 모든 핸들러를 초기화하고 단일 디스패치 라우트를 등록한다.
func SetupAll(e *echo.Echo, c *container.Container) {
	ctn = c
	for _, fn := range setupRegistry {
		fn(c)
	}
	e.POST("/api", dispatch)
}

// dispatch는 모든 게임 API 요청의 단일 진입점이다.
func dispatch(c echo.Context) error {
	raw, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return SendGameError(c, 0, CodeBadRequest, "failed to read body")
	}

	var req pb.GameRequest
	if err := proto.Unmarshal(raw, &req); err != nil {
		return SendGameError(c, 0, CodeBadRequest, "invalid envelope")
	}

	// 미들웨어(logger 등)에서 참조할 수 있도록 context에 저장한다.
	SetActionID(c, req.Action)
	SetUserID(c, req.UserId)
	SetClientVersion(c, req.ClientVersion)

	def, ok := actionRegistry[req.Action]
	if !ok {
		return SendGameError(c, req.Action, CodeBadRequest, "unknown action")
	}

	// client_version → design Catalog 라우팅. 모든 액션이 디자인 카탈로그에 접근하므로
	// 디스패처에서 1회 결정 후 UoW에 주입한다.
	catalog := ctn.DesignStore.GetByClientVersion(req.ClientVersion)
	if catalog == nil {
		return SendGameError(c, req.Action, CodeUnsupportedVersion, "unsupported client_version")
	}

	// 유저별 분산락 획득 (멀티 서버 동시 요청 직렬화).
	// userID가 0이면 (예: Login) 락 스킵.
	if req.UserId != 0 {
		token, err := ctn.UserLock.Acquire(c.Request().Context(), req.UserId)
		if err != nil {
			log.Warn().Uint64(log.KeyUserId, req.UserId).Uint32("action_id", req.Action).Msgf("lock acquire failed: %v", err)
			return SendGameError(c, req.Action, CodeBusy, "busy, retry later")
		}
		defer func() {
			_ = ctn.UserLock.Release(c.Request().Context(), req.UserId, token)
		}()
	}

	// UoW 생성 (catalog 포함) → context에 저장. 핸들러에서 꺼내 사용한다.
	u := uow.New(ctn, req.UserId, catalog)
	SetUoW(c, u)

	// 요청 body를 JSON으로 변환하여 context에 저장 (로깅용)
	if def.newReq != nil {
		reqMsg := def.newReq()
		if err := proto.Unmarshal(req.Body, reqMsg); err == nil {
			if jsonBytes, err := protojson.Marshal(reqMsg); err == nil {
				SetReqJSON(c, jsonBytes)
			}
		}
	}

	result, err := def.handler(c, req.Body)
	if err != nil {
		var ae *ActionError
		if errors.As(err, &ae) {
			log.Warn().Uint64(log.KeyUserId, req.UserId).Uint32("action_id", req.Action).Int32("code", ae.Code).Msgf("action error: %s", ae.Message)
			return SendGameError(c, req.Action, ae.Code, ae.Message)
		}
		log.Error().Uint64(log.KeyUserId, req.UserId).Uint32("action_id", req.Action).Msgf("internal error: %v", err)
		return SendGameError(c, req.Action, CodeInternalError, "internal error")
	}

	// 핸들러 성공 후 UoW 커밋
	if err := CommitOrRollback(u); err != nil {
		return SendGameError(c, req.Action, CodeInternalError, "commit failed")
	}

	// 응답을 JSON으로 변환하여 context에 저장 (로깅용)
	if resJSON, err := protojson.Marshal(result); err == nil {
		SetResJSON(c, resJSON)
	}

	body, err := proto.Marshal(result)
	if err != nil {
		return SendGameError(c, req.Action, CodeInternalError, "marshal error")
	}

	return sendGameResponse(c, req.Action, CodeOK, body)
}

func sendGameResponse(c echo.Context, action uint32, code int32, body []byte) error {
	resp := &pb.GameResponse{
		Action:    action,
		Code:      code,
		Timestamp: time.Now().Unix(),
		Body:      body,
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "application/x-protobuf", data)
}

func SendGameError(c echo.Context, action uint32, code int32, message string) error {
	errBody, _ := proto.Marshal(&pb.ErrorResponse{
		Code:    code,
		Message: message,
	})
	return sendGameResponse(c, action, code, errBody)
}

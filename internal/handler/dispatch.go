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
		return sendGameError(c, 0, http.StatusBadRequest, "failed to read body")
	}

	var req pb.GameRequest
	if err := proto.Unmarshal(raw, &req); err != nil {
		return sendGameError(c, 0, http.StatusBadRequest, "invalid envelope")
	}

	// 미들웨어(logger 등)에서 참조할 수 있도록 context에 저장한다.
	c.Set("action_id", req.Action)
	c.Set("user_id", req.UserId)

	def, ok := actionRegistry[req.Action]
	if !ok {
		return sendGameError(c, req.Action, http.StatusBadRequest, "unknown action")
	}

	// 유저별 분산락 획득 (멀티 서버 동시 요청 직렬화).
	// userID가 0이면 (예: Login) 락 스킵.
	if req.UserId != 0 {
		token, err := ctn.UserLock.Acquire(c.Request().Context(), req.UserId)
		if err != nil {
			log.Warn().Uint64(log.KeyUserId, req.UserId).Uint32("action_id", req.Action).Msgf("lock acquire failed: %v", err)
			return sendGameError(c, req.Action, CodeBusy, "busy, retry later")
		}
		defer func() {
			_ = ctn.UserLock.Release(c.Request().Context(), req.UserId, token)
		}()
	}

	// UoW 생성 → context에 저장. 핸들러에서 꺼내 사용한다.
	u := uow.New(ctn, req.UserId)
	c.Set("uow", u)

	// 요청 body를 JSON으로 변환하여 context에 저장 (로깅용)
	if def.newReq != nil {
		reqMsg := def.newReq()
		if err := proto.Unmarshal(req.Body, reqMsg); err == nil {
			if jsonBytes, err := protojson.Marshal(reqMsg); err == nil {
				c.Set("req_json", jsonBytes)
			}
		}
	}

	result, err := def.handler(c, req.Body)
	if err != nil {
		var ae *ActionError
		if errors.As(err, &ae) {
			log.Warn().Uint64(log.KeyUserId, req.UserId).Uint32("action_id", req.Action).Int32("code", ae.Code).Msgf("action error: %s", ae.Message)
			return sendGameError(c, req.Action, ae.Code, ae.Message)
		}
		log.Error().Uint64(log.KeyUserId, req.UserId).Uint32("action_id", req.Action).Msgf("internal error: %v", err)
		return sendGameError(c, req.Action, http.StatusInternalServerError, "internal error")
	}

	// 핸들러 성공 후 UoW 커밋
	if err := CommitOrRollback(u); err != nil {
		return sendGameError(c, req.Action, CodeInternalError, "commit failed")
	}

	// 응답을 JSON으로 변환하여 context에 저장 (로깅용)
	if resJSON, err := protojson.Marshal(result); err == nil {
		c.Set("res_json", resJSON)
	}

	body, err := proto.Marshal(result)
	if err != nil {
		return sendGameError(c, req.Action, http.StatusInternalServerError, "marshal error")
	}

	return sendGameResponse(c, req.Action, http.StatusOK, body)
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

func sendGameError(c echo.Context, action uint32, code int32, message string) error {
	errBody, _ := proto.Marshal(&pb.ErrorResponse{
		Code:    code,
		Message: message,
	})
	return sendGameResponse(c, action, code, errBody)
}

// UoW context에서 UnitOfWork를 꺼낸다. 핸들러에서 사용한다.
func UoW(c echo.Context) *uow.UnitOfWork {
	return c.Get("uow").(*uow.UnitOfWork)
}

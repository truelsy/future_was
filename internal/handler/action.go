package handler

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

// ActionHandler 디스패치되는 모든 핸들러의 시그니처이다.
// body는 GameRequest에서 추출한 inner protobuf 바이트를 담고 있다.
// 응답 proto 메시지 또는 에러를 반환한다.
type ActionHandler func(c echo.Context, body []byte) (proto.Message, error)

// actionDef는 액션 핸들러와 요청 메시지 팩토리를 묶는다.
type actionDef struct {
	handler ActionHandler
	newReq  func() proto.Message // 요청 proto 메시지 팩토리 (로깅용)
}

// ActionError 애플리케이션 레벨의 에러 코드를 담는다.
// 디스패처가 Code를 GameResponse.code에 사용한다.
type ActionError struct {
	Code    int32
	Message string
}

func (e *ActionError) Error() string { return e.Message }

// 액션 ID 상수 — 도메인별 그룹핑.
const (
	ActionLogin            uint32 = 1001
	ActionGetCards         uint32 = 2001
	ActionUpgradeCardLevel uint32 = 2002
)

// actionRegistry는 action_id → actionDef 매핑이다.
var actionRegistry = make(map[uint32]actionDef)

// RegisterAction 액션 핸들러를 등록한다. 각 핸들러 파일의 init()에서 호출된다.
// newReq는 요청 proto 메시지 팩토리로, 디스패처가 로깅 시 JSON 변환에 사용한다.
func RegisterAction(id uint32, fn ActionHandler, newReq func() proto.Message) {
	if _, exists := actionRegistry[id]; exists {
		panic("duplicate action id registration")
	}
	actionRegistry[id] = actionDef{handler: fn, newReq: newReq}
}

// Error 지정된 코드와 메시지로 ActionError를 생성한다.
func Error(code int32, msg string) *ActionError {
	return &ActionError{Code: code, Message: msg}
}

// Errorf 지정된 코드와 포맷 메시지로 ActionError를 생성한다.
func Errorf(code int32, format string, args ...any) *ActionError {
	return &ActionError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// BadRequest 400 에러를 생성한다.
func BadRequest(msg string) *ActionError {
	return &ActionError{Code: CodeBadRequest, Message: msg}
}

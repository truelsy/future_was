package handler

import (
	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/proto"
)

// ActionHandler 디스패치되는 모든 핸들러의 시그니처이다.
// body는 GameRequest에서 추출한 inner protobuf 바이트를 담고 있다.
// 디자인 Catalog은 UoW를 통해 접근한다 (handler.UoW(c).Catalog()).
// 응답 proto 메시지 또는 에러를 반환한다.
//
// 에러 반환 시 *errcode.Error를 사용하면 dispatcher가 envelope code로 매핑한다.
type ActionHandler func(c echo.Context, body []byte) (proto.Message, error)

// actionDef는 액션 핸들러와 요청 메시지 팩토리를 묶는다.
type actionDef struct {
	handler ActionHandler
	newReq  func() proto.Message // 요청 proto 메시지 팩토리 (로깅용)
	noAuth  bool                 // true면 디스패처의 세션 검증을 스킵한다 (예: Login).
}

// 액션 ID 상수 — 도메인별 그룹핑.
const (
	ActionLogin uint32 = 1001

	ActionGetCards         uint32 = 2001
	ActionUpgradeCardLevel uint32 = 2002

	ActionGetItems uint32 = 3001

	ActionGetAssets uint32 = 4001
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

// RegisterNoAuthAction 세션 검증을 면제할 액션을 등록한다.
// 로그인처럼 토큰 발급 전에 호출되어야 하는 액션에만 사용한다.
func RegisterNoAuthAction(id uint32, fn ActionHandler, newReq func() proto.Message) {
	if _, exists := actionRegistry[id]; exists {
		panic("duplicate action id registration")
	}
	actionRegistry[id] = actionDef{handler: fn, newReq: newReq, noAuth: true}
}

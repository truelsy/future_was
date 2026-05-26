package handler

import (
	"future_cpbl_web_server/internal/uow"

	"github.com/labstack/echo/v4"
)

// echo.Context에 저장하는 요청 단위 데이터의 키.
// 매직 스트링 분산 + 오타 시 컴파일 에러 미발생 문제를 차단하기 위해 상수화한다.
// 외부에서는 SetXxx/Xxx 헬퍼만 사용한다.
const (
	ctxKeyActionID      = "action_id"
	ctxKeyUserID        = "user_id"
	ctxKeyClientVersion = "client_version"
	ctxKeyUoW           = "uow"
	ctxKeyReqJSON       = "req_json"
	ctxKeyResJSON       = "res_json"
)

// ----- Setter (디스패처 전용) -----

func SetActionID(c echo.Context, v uint32)        { c.Set(ctxKeyActionID, v) }
func SetUserID(c echo.Context, v uint64)          { c.Set(ctxKeyUserID, v) }
func SetClientVersion(c echo.Context, v string)   { c.Set(ctxKeyClientVersion, v) }
func SetUoW(c echo.Context, u *uow.UnitOfWork)    { c.Set(ctxKeyUoW, u) }
func SetReqJSON(c echo.Context, b []byte)         { c.Set(ctxKeyReqJSON, b) }
func SetResJSON(c echo.Context, b []byte)         { c.Set(ctxKeyResJSON, b) }

// ----- Getter (핸들러/미들웨어용) -----

// ActionID 디스패처가 설정한 액션 ID. 미설정 시 0.
func ActionID(c echo.Context) uint32 {
	if v, ok := c.Get(ctxKeyActionID).(uint32); ok {
		return v
	}
	return 0
}

// UserID 디스패처가 설정한 요청 유저 ID. 미설정 시 0 (envelope 디코딩 전 등).
func UserID(c echo.Context) uint64 {
	if v, ok := c.Get(ctxKeyUserID).(uint64); ok {
		return v
	}
	return 0
}

// ClientVersion 디스패처가 설정한 클라이언트 버전. 미설정 시 빈 문자열.
func ClientVersion(c echo.Context) string {
	if v, ok := c.Get(ctxKeyClientVersion).(string); ok {
		return v
	}
	return ""
}

// UoW 디스패처가 생성한 UnitOfWork. 디스패처를 거치지 않은 컨텍스트에서는 nil.
// 정상 dispatched 핸들러에서는 항상 non-nil이 보장된다.
func UoW(c echo.Context) *uow.UnitOfWork {
	if v, ok := c.Get(ctxKeyUoW).(*uow.UnitOfWork); ok {
		return v
	}
	return nil
}

// ReqJSON 로깅용 요청 protobuf의 JSON 변환 결과. 미설정 시 nil.
func ReqJSON(c echo.Context) []byte {
	if v, ok := c.Get(ctxKeyReqJSON).([]byte); ok {
		return v
	}
	return nil
}

// ResJSON 로깅용 응답 protobuf의 JSON 변환 결과. 미설정 시 nil.
func ResJSON(c echo.Context) []byte {
	if v, ok := c.Get(ctxKeyResJSON).([]byte); ok {
		return v
	}
	return nil
}

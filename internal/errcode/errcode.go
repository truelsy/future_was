// Package errcode 도메인 에러 코드와 *Error 타입을 정의한다.
//
// service 등 비즈니스 로직 계층은 handler(interface adapter)에 의존하지 않고
// 이 패키지만 import해서 에러를 생성한다. handler/dispatch는 errors.As로
// *errcode.Error를 추출해 envelope 응답에 매핑한다.
//
// Clean Architecture의 의존성 방향: service → errcode (안쪽), handler → errcode (안쪽).
// 즉 service ↛ handler를 깨기 위한 공통 의존 지점.
package errcode

import "fmt"

// ---------------------------------------------------------------------------
// 에러 코드 정의
// ---------------------------------------------------------------------------
//
// 코드 체계:
//   200       : 성공
//   400       : 잘못된 요청 (파라미터 누락, 형식 오류)
//   429       : 과도 요청
//   500       : 서버 내부 에러
//   1xxx      : 계정 도메인
//   2xxx      : 카드 도메인
//   3xxx      : 에셋 도메인
//   4xxx      : 디자인/버전
//   5xxx      : 운영 상태 (점검 등)
//
// 새 도메인 추가 시 해당 대역의 코드를 정의한다.

const (
	// 공통
	CodeOK            int32 = 200
	CodeBadRequest    int32 = 400
	CodeUnauthorized  int32 = 401
	CodeBusy          int32 = 429
	CodeInternalError int32 = 500

	// 계정 (1xxx)
	CodeAccountNotFound int32 = 1001
	CodeAccountInactive int32 = 1002

	// 카드 (2xxx)
	CodeCardNotFound int32 = 2001

	// 에셋 (3xxx)
	CodeAssetNotFound     int32 = 3001
	CodeAssetInsufficient int32 = 3002

	// 디자인/버전 (4xxx)
	CodeUnsupportedDesignVersion int32 = 4001
	CodeNotFoundDesign           int32 = 4002

	// 운영 상태 (5xxx)
	CodeMaintenance int32 = 5001
)

// Error 애플리케이션 레벨의 코드 + 메시지 에러.
// handler/dispatch가 errors.As로 추출해 envelope 응답에 매핑한다.
type Error struct {
	Code    int32
	Message string
}

func (e *Error) Error() string { return e.Message }

// New 지정된 코드와 메시지로 Error를 생성한다.
func New(code int32, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// Newf 지정된 코드와 포맷 메시지로 Error를 생성한다.
func Newf(code int32, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// BadRequest 400 에러를 생성한다.
func BadRequest(msg string) *Error {
	return &Error{Code: CodeBadRequest, Message: msg}
}

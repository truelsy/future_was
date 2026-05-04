package handler

// ---------------------------------------------------------------------------
// 에러 코드 정의
// ---------------------------------------------------------------------------
//
// 코드 체계:
//   200       : 성공
//   400       : 잘못된 요청 (파라미터 누락, 형식 오류)
//   500       : 서버 내부 에러
//   1xxx      : 계정 도메인
//   2xxx      : 카드 도메인
//   3xxx      : 에셋 도메인
//
// 새 도메인 추가 시 해당 대역의 코드를 정의한다.

const (
	// 공통
	CodeOK            int32 = 200
	CodeBadRequest    int32 = 400
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
)

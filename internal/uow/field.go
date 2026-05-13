package uow

// 캐시 필드명 상수. 서비스/핸들러에서 LoadOne / LoadList / SetField를
// 래퍼 없이 직접 호출할 때 참조한다.
//
// 버전 접미사(`.vN`) 컨벤션:
//
//	해당 도메인 모델(struct)의 스키마가 호환 안 되게 변경되면 버전을 올린다.
//	예: model.Card에 새 필드 추가 / 타입 변경 → FieldCards = "cards.v2"
//	배포 즉시 모든 서버가 새 키로 조회 → cache miss → DB 재로드 → 새 키로 저장.
//	옛 키(예: "cards.v1")는 30분 TTL로 자연 소멸하므로 별도 정리 불필요.
//
// 호환되는 변경(필드 제거 등)은 버전 유지 가능. 자세한 정책은 plan 파일 참조.
const (
	// user scope (user:{userID} Hash 필드)
	FieldAccount = "account.v1"
	FieldAssets  = "assets.v1"
	FieldCards   = "cards.v1"
	FieldItems   = "items.v1"

	// club scope (club:{clubID} Hash 필드)
	FieldClubInfo    = "info.v1"
	FieldClubMembers = "members.v1"
)

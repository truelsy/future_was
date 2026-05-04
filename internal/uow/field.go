package uow

// 캐시 필드명 상수. 서비스/핸들러에서 LoadOne / LoadList / SetField를
// 래퍼 없이 직접 호출할 때 참조한다.
const (
	FieldAccount = "account"
	FieldAssets  = "assets"
	FieldCards   = "cards"
)

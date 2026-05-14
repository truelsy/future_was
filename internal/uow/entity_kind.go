package uow

// 버전 접미사(`.vN`) 컨벤션:
//
//	해당 도메인 모델(struct)의 스키마가 호환 안 되게 변경되면 버전을 올린다.
//	예: model.Card에 새 필드 추가 / 타입 변경 → EntityCards = EntityKind{Name: "cards.v2", ...}
//	배포 즉시 모든 서버가 새 키로 조회 → cache miss → DB 재로드 → 새 키로 저장.
//	옛 키(예: "cards.v1")는 30분 TTL로 자연 소멸하므로 별도 정리 불필요.
//

// EntityKind 도메인 엔티티의 종류를 식별한다.
// store/캐시 key 이름(Name), 소유 scope(Owner), 라우팅 대상 DB(IsGameDB)를
// 한 묶음으로 들고 있어 LoadOne/LoadList/Create/CreateNow/Update 호출 시
// owner·DB 라우팅이 자동으로 결정된다.
type EntityKind struct {
	Name     string
	Owner    Owner
	IsGameDB bool
}

// user scope
var (
	EntityAccount = EntityKind{Name: "account.v1", Owner: OwnerUser, IsGameDB: true}
	EntityAssets  = EntityKind{Name: "assets.v1", Owner: OwnerUser, IsGameDB: false}
	EntityCards   = EntityKind{Name: "cards.v1", Owner: OwnerUser, IsGameDB: false}
	EntityItems   = EntityKind{Name: "items.v1", Owner: OwnerUser, IsGameDB: false}
)

// club scope
var (
	EntityClubInfo    = EntityKind{Name: "info.v1", Owner: OwnerClub, IsGameDB: true}
	EntityClubMembers = EntityKind{Name: "members.v1", Owner: OwnerClub, IsGameDB: true}
)

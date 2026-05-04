# Future Next Baseball — Game Server

Go 기반 야구 게임 서버. Echo + MySQL (Shard) + Redis.

## 빌드 & 실행

```bash
make build    # 서버 바이너리 빌드
make run      # 서버 실행
make proto    # protobuf 코드 재생성
make vet      # 정적 분석
make client   # 테스트 클라이언트 실행
make clean    # 빌드 산출물 삭제
```

## 디렉토리 구조

```
.
├── main.go                  # 서버 엔트리포인트
├── config/                  # YAML 설정 파싱
├── router/                  # 핸들러 초기화 + 디스패치 라우트 등록
├── proto/                   # .proto 원본
├── pb/                      # protoc 생성 코드 (수동 편집 금지)
├── cmd/client/              # 인터랙티브 테스트 클라이언트
├── internal/
│   ├── app/                 # 앱 파라미터 (project, stage, hostname)
│   ├── log/                 # zerolog 래퍼 + 로그 필드 상수
│   ├── middleware/          # 요청 로깅, 패닉 복구
│   ├── container/           # 공유 의존성 (GameDB, UserCache)
│   ├── database/            # sqlx 래퍼, Shard 레지스트리, Model 인터페이스
│   ├── cache/               # Redis 초기화 + 유저별 Hash 캐시
│   ├── model/               # 도메인 모델 (DB 테이블 매핑, JSONField 래퍼)
│   ├── repository/          # DB 직접 조회 (UoW 밖 독립 조회용)
│   ├── uow/                 # Unit of Work (지연 로딩, 쓰기 큐잉, 커밋)
│   ├── service/             # 비즈니스 로직 (도메인별 서비스)
│   ├── handler/             # 디스패처, 액션 ID, 에러 코드
│   │   ├── account/         #   계정 도메인 핸들러
│   │   └── card/            #   카드 도메인 핸들러
│   └── util/                # 범용 유틸리티 (변환, 수학, 시간)
└── Makefile
```

## 요청 흐름

```
Client
  │  GameRequest { action, user_id, timestamp, body }
  ▼
Echo (POST /api)
  │
  ▼
dispatch.go
  ├─ envelope unmarshal (GameRequest)
  ├─ context에 action_id, user_id 저장
  ├─ UoW 생성 → context에 저장
  ├─ actionRegistry에서 핸들러 조회
  ├─ 요청 body → JSON 변환 (로깅용)
  ▼
ActionHandler (예: card/get_card_list.go)
  ├─ handler.UoW(c)로 context에서 UoW 조회
  ├─ Service 호출
  │    ├─ UoW.Cards() → LoadList (store → Redis → DB)
  │    ├─ 비즈니스 로직 수행
  │    └─ uow.Update / uow.Create
  ▼
dispatch.go (핸들러 반환 후)
  ├─ CommitOrRollback(u) — 자동 실행
  │    ├─ 성공: DB 트랜잭션 실행 → 캐시 갱신
  │    └─ 실패: 캐시 무효화 → 에러 응답
  ├─ 응답 body → JSON 변환 (로깅용)
  └─ GameResponse { action, code, timestamp, body }
       │
       ▼
     Client
```

## 프로토콜

단일 엔드포인트 `POST /api`에 envelope protobuf로 통신한다.

```protobuf
// 요청
GameRequest { action: uint32, user_id: uint64, timestamp: int64, body: bytes }

// 응답
GameResponse { action: uint32, code: int32, timestamp: int64, body: bytes }
```

- `action`: 액션 ID (1001 = Login, 2001 = GetCards, ...)
- `user_id`: 요청 유저 (로그인 제외)
- `body`: inner protobuf (액션별 Request/Response 메시지)
- `code`: 200 성공, 400 잘못된 요청, 500 내부 에러, 도메인 에러 (1xxx~)
- `timestamp`: 서버 응답 시각 (Unix epoch seconds)

## 에러 코드 체계

| 대역 | 도메인 | 예시 |
|------|--------|------|
| 200 | 성공 | |
| 400 | 잘못된 요청 | 파라미터 누락 |
| 500 | 서버 내부 에러 | DB 장애, 커밋 실패 |
| 1xxx | 계정 | 1001 계정 미존재, 1002 비활성 |
| 2xxx | 카드 | 2001 카드 미존재 |
| 3xxx | 에셋 | 3001 에셋 미존재, 3002 잔액 부족 |

새 도메인 추가 시 다음 대역 (4xxx, 5xxx, ...) 사용.

## 액션 ID 체계

| 대역 | 도메인 |
|------|--------|
| 1001~1999 | 계정 |
| 2001~2999 | 카드 |
| 3001~3999 | 에셋 |

## 새 도메인 추가 가이드

예: `Item` 도메인 추가 시

### 1. Model 정의

```go
// internal/model/item.go
type Item struct {
    Idx        uint64 `db:"idx" json:"idx"`
    UserID     uint64 `db:"user_id" json:"user_id"`
    ItemID     uint32 `db:"item_id" json:"item_id"`
    Quantity   int64  `db:"quantity" json:"quantity"`
    InsertTime uint32 `db:"insert_time" json:"insert_time"`
    UpdateTime uint32 `db:"update_time" json:"update_time"`
}

func (*Item) TableName() string         { return "TB_ITEM" }
func (*Item) PrimaryKey() string        { return "idx" }
func (i *Item) SetPrimaryKey(id int64)  { i.Idx = uint64(id) }
```

### 2. 캐시 필드 상수 추가

```go
// internal/uow/field.go
const (
    FieldItems = "items"
)
```

### 3. UoW 편의 래퍼 추가 (선택)

```go
// internal/uow/uow.go — 래퍼 없이 서비스에서 직접 호출해도 됨
func (u *UnitOfWork) Items() ([]*model.Item, error) {
    return LoadList[*model.Item](u, FieldItems, u.ShardDB())
}
```

또는 서비스에서 직접:
```go
items, err := uow.LoadList[*model.Item](u, uow.FieldItems, u.ShardDB())
```

### 4. Service 작성

```go
// internal/service/item_service.go
type ItemService struct{}

func NewItemService() *ItemService { return &ItemService{} }

func (s *ItemService) GetItem(u *uow.UnitOfWork, itemID uint32) (*model.Item, error) {
    items, err := u.Items()
    if err != nil { return nil, err }
    for _, item := range items {
        if item.ItemID == itemID { return item, nil }
    }
    return nil, nil
}
```

### 5. Proto 정의

```protobuf
// proto/item.proto
message GetItemsRequest {}
message GetItemsResponse { repeated ItemData items = 1; }
```

```bash
make proto
```

### 6. 액션 ID + 에러 코드 등록

```go
// internal/handler/action.go
const ActionGetItems uint32 = 4001

// internal/handler/error_code.go
const CodeItemNotFound int32 = 4001
```

### 7. Handler 작성

```go
// internal/handler/item/setup.go
package item

func init() { handler.RegisterSetup(setupItemHandler) }

func setupItemHandler(c *container.Container) {
    svc := service.NewItemService()
    h := &itemHandler{svc: svc}
    handler.RegisterAction(handler.ActionGetItems, h.GetItems,
        func() proto.Message { return &pb.GetItemsRequest{} })
}

type itemHandler struct {
    svc *service.ItemService
}
```

```go
// internal/handler/item/get_items.go
func (h *itemHandler) GetItems(c echo.Context, _ []byte) (proto.Message, error) {
    u := handler.UoW(c)  // dispatch에서 생성된 UoW를 context에서 조회
    items, err := h.svc.GetItems(u)
    if err != nil { return nil, err }
    // ... pb 변환 후 반환
    // CommitOrRollback은 dispatch에서 자동 호출되므로 핸들러에서 호출하지 않는다.
}
```

### 8. Router에 blank import 추가

```go
// router/router.go
import (
    _ "future_next_baseball/internal/handler/item"
)
```

### 9. 테스트 클라이언트에 액션 추가 (선택)

```go
// cmd/client/main.go — actions 슬라이스에 항목 추가
```

### 체크리스트

- [ ] `internal/model/item.go` — struct + TableName + PrimaryKey + SetPrimaryKey
- [ ] `internal/uow/field.go` — `FieldItems` 상수
- [ ] `internal/service/item_service.go` — 비즈니스 로직
- [ ] `proto/item.proto` — Request/Response 메시지 → `make proto`
- [ ] `internal/handler/action.go` — 액션 ID 상수
- [ ] `internal/handler/error_code.go` — 에러 코드 상수
- [ ] `internal/handler/item/setup.go` — init + 액션 등록
- [ ] `internal/handler/item/get_items.go` — 핸들러 로직
- [ ] `router/router.go` — blank import
- [ ] `cmd/client/main.go` — 테스트 액션 (선택)

# Future Next Baseball — Game Server

Go 기반 야구 게임 서버. Echo + MySQL (Shard) + Redis 다중 인스턴스 + CDN 기반 디자인 데이터.

## 빌드 & 실행

```bash
make build         # 서버 바이너리 빌드
make run           # 서버 실행
make proto         # protobuf 코드 재생성
make vet           # 정적 분석
make client        # 인터랙티브 테스트 클라이언트
make design        # Excel → JSON 변환 (서버용 / TARGET=client 가능)
make design-struct # Excel → Go struct (서버 전용)
make clean         # 빌드 산출물 삭제
```

## 디렉토리 구조

```
.
├── main.go                  # 서버 엔트리포인트
├── config/                  # YAML 설정 파싱
├── router/                  # 디스패치 라우트 + Admin API
├── proto/                   # .proto 원본
├── pb/                      # protoc 생성 코드 (수정 금지)
├── cmd/client/              # 인터랙티브 테스트 클라이언트
├── tools/
│   ├── excel2json/          # Excel → JSON (CDN 업로드용)
│   └── excel2struct/        # Excel → Go struct (서버 전용)
├── internal/
│   ├── app/                 # 앱 파라미터
│   ├── log/                 # zerolog 래퍼
│   ├── middleware/          # 요청 로깅, 패닉 복구
│   ├── container/           # 공유 의존성 주입
│   ├── database/            # sqlx 래퍼, Shard 레지스트리, Model 인터페이스
│   ├── cache/               # Redis 다중 인스턴스 + UserCache + UserLock
│   ├── model/               # 도메인 모델 (DB 매핑, JSONField)
│   ├── repository/          # DB 직접 조회
│   ├── uow/                 # Unit of Work (지연 로딩, 쓰기 큐잉, Catalog 보관)
│   ├── service/             # 비즈니스 로직
│   ├── design/              # 디자인 데이터 (CDN 다운로드 + 메모리 Catalog)
│   │   └── schema/          #   excel2struct 자동 생성 (수정 금지)
│   ├── handler/             # 디스패처, 액션 ID, 에러 코드
│   │   ├── account/
│   │   └── card/
│   └── util/
└── Makefile
```

## 요청 흐름

```
Client
  │  GameRequest { action, user_id, timestamp, client_version, body }
  ▼
Echo (POST /api)
  │
  ▼
dispatch.go
  ├─ envelope unmarshal → action_id / user_id / client_version 결정
  ├─ DesignStore.GetByClientVersion → Catalog 결정 (없으면 4001 에러)
  ├─ UserLock 획득 (멀티 서버 직렬화, userID≠0일 때)
  ├─ UoW 생성 (userID + Catalog 포함) → context에 저장
  ▼
ActionHandler(c, body)
  ├─ u := handler.UoW(c)
  ├─ u.Catalog().BatData().Get(...)   ← 디자인 데이터 접근
  ├─ Service 호출 (지연 로딩 + 쓰기 큐잉)
  ▼
dispatch.go (핸들러 반환 후)
  ├─ CommitOrRollback(u) 자동 실행 (DB 트랜잭션 + UserCache 갱신)
  └─ GameResponse { action, code, timestamp, body }
       ▼
     Client
```

## 프로토콜

단일 엔드포인트 `POST /api`에 envelope protobuf로 통신.

```protobuf
message GameRequest {
  uint32 action          = 1;
  uint64 user_id         = 2;
  int64  timestamp       = 3;
  string client_version  = 4;   // → 서버에서 server_version으로 매핑
  bytes  body            = 5;
}

message GameResponse {
  uint32 action     = 1;
  int32  code       = 2;        // 200/4xx/5xx + 도메인 코드
  int64  timestamp  = 3;
  bytes  body       = 4;
}
```

## 에러 코드 체계

| 대역 | 도메인 | 예시 |
|------|--------|------|
| 200 | 성공 | |
| 400 | 잘못된 요청 | 파라미터 누락 |
| 429 | 과도 요청 | 분산락 대기 초과 |
| 500 | 서버 내부 에러 | DB 장애, 커밋 실패 |
| 1xxx | 계정 | 1001 계정 미존재 |
| 2xxx | 카드 | 2001 카드 미존재 |
| 3xxx | 에셋 | 3001 미존재, 3002 잔액 부족 |
| 4xxx | 디자인/버전 | 4001 미지원 client_version |

## 액션 ID 체계

| 대역 | 도메인 |
|------|--------|
| 1001~1999 | 계정 |
| 2001~2999 | 카드 |
| 3001~3999 | 에셋 |

---

## 디자인 데이터 시스템

서버는 TB_VERSION 테이블을 기준으로 **활성 server_version의 최신 N개**를 메모리에 보관합니다 (현재 `MaxActiveVersions = 2` — 현재 + 직전).

### 핵심 객체

| 타입 | 역할 |
|------|------|
| `design.Design[K,V]` | 도메인 1개의 PK 인덱스 (제네릭 — `Get/Find/All/Len`) |
| `design.Catalog` | 한 server_version의 모든 도메인 묶음. 도메인별 접근자 노출 |
| `design.Store` | `map[client_version]*Catalog` 보관, 원자 교체 |
| `design.Loader` | CDN에서 manifest + JSON 다운로드 + 체크섬 검증 |
| `design.Syncer` | TB_VERSION 조회 + Catalog 갱신 + Pub/Sub |

### Catalog 사용 패턴

```go
// 어디서든 (handler/service) UoW를 통해 접근
c := u.Catalog()

// PK 단건 조회
batter := c.BatData().Get(nid)        // *schema.BatDataDesign or nil
if currency, ok := c.Currency().Find(itemID); ok { ... }

// 보조 인덱스 (load time에 별도 구축)
batter := c.BatDataByUseFlag().Get(nid)

// 전체 순회
for _, v := range c.BatData().All() { ... }
```

### 비-PK 조회 전략

| 케이스 | 방법 |
|--------|------|
| 데이터 < 500 row | 도메인 헬퍼에 선형 탐색 |
| 자주 조회 + 큰 데이터 | **보조 인덱스** — load time에 `*Design[K2, V]` 추가 (`batDataByUseFlag` 패턴) |
| 1:N 관계 | `*Design[K, []V]` |
| 범위/조건 | 정렬 슬라이스 + `sort.Search` |

도메인별 비-PK 헬퍼는 `internal/design/<domain>.go`에 작성합니다 (예: `currency_list.go`).

---

## Excel 포맷 (디자인 데이터)

xlsx 파일은 4행 헤더 구조를 따릅니다.

| 행 | 의미 | 예 |
|----|------|------|
| 1행 | CLIENT 사용 여부 | `CLIENT` 또는 빈 셀 |
| 2행 | SERVER 사용 여부 | `SERVER` 또는 빈 셀 |
| 3행 | 데이터 타입 | `INT`, `STRING`, `BOOL`, `INT(PK)` ... |
| 4행 | 컬럼명 | `IDX`, `ITEM_ID`, ... |
| 5행~ | 데이터 | |

### 타입

| Excel 타입 | Go 타입 | JSON 빈 셀 기본값 |
|-----------|--------|------------------|
| `STRING`/`STR`/`TEXT` | `string` | `""` |
| `INT`/`INT32` | `int32` | `0` |
| `INT64`/`LONG` | `int64` | `0` |
| `UINT`/`UINT32` | `uint32` | `0` |
| `FLOAT`/`FLOAT32` | `float32` | `0.0` |
| `FLOAT64`/`DOUBLE` | `float64` | `0.0` |
| `BOOL`/`BOOLEAN` | `bool` | `false` |

### PK

3행 타입 셀에 **`(PK)` 접미사**가 붙은 컬럼이 PK입니다 (예: `INT(PK)`).
**파일당 정확히 1개**의 PK가 필요합니다 (검증 실패 시 변환 거부).

### 호환성 규칙

- **컬럼 추가는 자유**, 삭제/이름 변경/타입 변경 금지
- 사용 안 하는 컬럼은 1행/2행 마커를 비워두면 자동 제외 (deprecated 처리)

---

## 디자인 데이터 패치 가이드

### 전체 흐름

```
[기획자] design-data 저장소에 xlsx 작업 → git push
   │
   ▼
[CI 파이프라인]
   1. excel2json --target server  → JSON + manifest.json (sha256)
   2. 버전 결정 (커밋 메시지 [release vX.Y.Z] 또는 자동)
   3. CDN 업로드 → /design/v1.0.5/
   ▼
[운영자] TB_VERSION 갱신 (is_active=1)
   ▼
[운영자 또는 CI] POST /admin/design/reload (한 서버에만 호출)
   ▼
[Redis Pub/Sub] 모든 서버가 자동 동기 갱신
```

### 1단계: Excel 작업 + Git push

```bash
# design-data/ 저장소
git pull
# Excel 수정...
git commit -m "[release v1.0.5] 신규 카드 10종 추가"
git push
```

### 2단계: CI 변환 + CDN 업로드

```bash
# 서버용 JSON 생성
python tools/excel2json/excel2json.py --target server --version v1.0.5

# 출력: output/v1.0.5/{*.json, manifest.json}
# manifest.json: { version, target, files: [{name, checksum, pk}] }

# CDN 업로드 (예시)
aws s3 sync output/v1.0.5/ s3://cdn-bucket/design/v1.0.5/
```

> 서버용 struct 코드는 `make design-struct`로 별도 생성하여 서버 저장소에 커밋합니다 (`internal/design/schema/`).

### 3단계: TB_VERSION 갱신

```sql
INSERT INTO TB_VERSION
  (client_version, server_version, app_id, is_active, ...)
VALUES
  ('1.2.0', 'v1.0.5', 'AOS', 1, ...),
  ('1.2.0', 'v1.0.5', 'IOS', 1, ...);

-- (선택) 오래된 버전 비활성화
UPDATE TB_VERSION SET is_active=0 WHERE server_version='v1.0.3';
```

> 서버는 `is_active=1` 행을 `server_version DESC`로 조회 → 최신 2개 server_version만 메모리에 유지.
> 같은 server_version에 매핑된 여러 client_version은 **동일 Catalog 포인터를 공유**합니다.

### 4단계: Reload 호출

```bash
curl -X POST http://server:8089/admin/design/reload
```

호출받은 서버:
1. TB_VERSION 다시 조회 → 활성 server_version 결정
2. **기존 Catalog 재사용** + 신규 server_version만 CDN 다운로드
3. 새 `client_version → Catalog` 매핑 원자 교체
4. **Redis PUBLISH** → 다른 서버들도 자동 동기 갱신

### 5단계: 검증

```bash
# 클라이언트로 새 client_version 요청
make client    # [v]로 client_version 변경 후 액션 호출

# 서버 로그
# "design loaded: v1.0.5"
# "design active versions: [v1.0.5 v1.0.4]"
```

### 단계별 책임

| 단계 | 담당 | 작업 |
|------|------|------|
| 1 | 기획자 | Excel 작업 → `git push` |
| 2 | CI (자동) | JSON 변환 + manifest + CDN 업로드 |
| 3 | 운영자 | TB_VERSION SQL |
| 4 | 운영자/CI | `POST /admin/design/reload` |
| 5 | 모든 서버 | Pub/Sub 자동 동기 |

---

## 새 디자인 도메인 추가

예: `Item` 디자인 데이터 추가

### 1. Excel 파일 작성 (`design-data/ITEM.xlsx`)
4행 헤더 포맷, PK 컬럼 1개, 서버 사용 컬럼만 SERVER 마커.

### 2. Go struct 생성

```bash
make design-struct
# → internal/design/schema/item_design.go
```

### 3. Catalog에 도메인 추가

```go
// internal/design/store.go
type Catalog struct {
    Version  string
    batData  *Design[uint32, *schema.BatDataDesign]
    currency *Design[int32, *schema.CurrencyListDesign]
    item     *Design[uint32, *schema.ItemDesign]   // 추가
}

func NewCatalog(v string) *Catalog {
    return &Catalog{
        // ...
        item: NewDesign[uint32, *schema.ItemDesign](),
    }
}

func (c *Catalog) Item() *Design[uint32, *schema.ItemDesign] { return c.item }
```

### 4. Loader에 unmarshal case 추가

```go
// internal/design/loader.go의 unmarshalInto()
case "ITEM.json":
    var list []*schema.ItemDesign
    if err := json.Unmarshal(data, &list); err != nil { return err }
    for _, v := range list {
        c.item.set(v.Idx, v)
    }
```

### 5. (선택) 비-PK 헬퍼

```go
// internal/design/item.go (도메인별 헬퍼 파일)
func (c *Catalog) FindItemByCategory(cat string) []*schema.ItemDesign {
    var out []*schema.ItemDesign
    for _, v := range c.item.All() {
        if v.Category == cat { out = append(out, v) }
    }
    return out
}
```

### 6. CDN/CI 흐름은 동일

자동 생성 파일(`schema/*_design.go`, `pb/*.pb.go`)은 절대 수정하지 않습니다.

---

## 새 비즈니스 도메인 추가 (DB 기반)

예: `Item` 도메인 (TB_ITEM 테이블)

### 체크리스트

- [ ] `internal/model/item.go` — struct + `TableName/PrimaryKey/SetPrimaryKey/IsSingleton`
- [ ] `internal/uow/field.go` — `FieldItems` 상수
- [ ] `internal/uow/uow.go` — (선택) `(u *UoW) Items()` 편의 래퍼
- [ ] `internal/service/item_service.go` — 비즈니스 로직 (UoW 인자)
- [ ] `proto/item.proto` → `make proto`
- [ ] `internal/handler/action.go` — 액션 ID 상수 (4001~)
- [ ] `internal/handler/error_code.go` — 에러 코드 상수
- [ ] `internal/handler/item/setup.go` — `init()` + `RegisterAction`
- [ ] `internal/handler/item/<action>.go` — 핸들러 로직
- [ ] `router/router.go` — blank import 추가
- [ ] `cmd/client/main.go` — 테스트 액션 (선택)

### 핸들러 시그니처

```go
func (h *itemHandler) GetItems(c echo.Context, body []byte) (proto.Message, error) {
    u := handler.UoW(c)
    items, err := h.svc.GetItems(u)
    if err != nil { return nil, err }
    // 디자인 데이터가 필요하면 u.Catalog().Item().Get(id)
    // CommitOrRollback은 dispatch가 자동 호출
    return &pb.GetItemsResponse{...}, nil
}
```

---

## Config (`config.yaml`)

```yaml
server:
  port: "8089"

databases:
  - { name: game,    shard_id: 0,  host: localhost, port: 3306, user: root, password: ..., dbname: FUTURE_NPB_GAME }
  - { name: shard_1, shard_id: 10, host: localhost, port: 3306, user: root, password: ..., dbname: FNPB_GAME_S0 }

redis:
  - { name: user_lock,   host: localhost, port: 6379, password: ..., db: 0 }
  - { name: user_cache,  host: localhost, port: 6379, password: ..., db: 1 }
  - { name: design_sync, host: localhost, port: 6379, password: ..., db: 3 }

cdn:
  design_base_url: "https://example.com/design"
  http_timeout_seconds: 10
```

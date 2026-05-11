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

## Unit of Work (UoW) 패턴

### 한 줄 요약
**한 요청에서 일어나는 모든 DB 읽기/쓰기를 모았다가, 마지막에 한 번에 트랜잭션으로 커밋한다.**

### 왜 필요한가
서비스 코드가 매번 직접 DB·캐시·트랜잭션을 다루면:
- 같은 데이터를 한 요청에서 여러 번 조회하는 낭비 (예: `Account`을 3개 서비스에서 각자 SELECT)
- 캐시 갱신 누락
- 부분 실패 시 일관성 깨짐

UoW가 이 모든 것을 **요청 단위로 한 번만** 처리해 준다.

### 컨셉

```
서비스가 "지금 카드 데이터 줘"라고 하면
   └─ UoW가 알아서:
        1) 메모리 store에 있어? → 있으면 그대로 반환
        2) Redis 캐시에 있어? → 있으면 store에 채우고 반환
        3) 둘 다 없어? → DB SELECT → 캐시 + store에 저장 → 반환

서비스가 "이 카드 레벨 +1 해줘"라고 하면
   └─ UoW가:
        1) store의 카드를 즉시 수정 (이후 동일 요청 내 모든 조회는 변경분 반영)
        2) "UPDATE 큐"에 작업만 등록 (DB는 아직 안 건드림)

요청 끝(dispatch가 자동 호출):
   └─ UoW.Commit():
        1) 큐에 쌓인 모든 쓰기를 DB별 트랜잭션으로 실행
        2) 성공 시 캐시(Redis Hash)에 일괄 반영
        3) 실패 시 캐시 무효화 (DeleteAll)
```

### 핵심 메서드

| 메서드 | 동작 |
|--------|------|
| `LoadOne[T](u, field, db)` | 단일 엔티티 지연 로딩: store → Redis → DB |
| `LoadList[T](u, field, db)` | 슬라이스 지연 로딩 |
| `Create[T](u, field, m, db)` | INSERT 큐잉. PK는 `Commit()` 후 반영 |
| `CreateNow[T](u, field, m, db)` | 즉시 INSERT (PK가 바로 필요할 때, 예: 신규 계정 생성) |
| `Update[T](u, m, db, cols...)` | UPDATE 큐잉 |
| `u.Commit()` | 모든 ops를 DB별 트랜잭션 실행 + 캐시 반영 |
| `u.Account()` / `u.Cards()` / `u.Assets()` | 도메인별 편의 래퍼 (LoadOne/LoadList 호출) |
| `u.Catalog()` | 디자인 데이터 카탈로그 |
| `u.ShardDB()` | 유저의 Account.DBShardID로 라우팅된 shard |

### 사용 예시

```go
// 핸들러에서
u := handler.UoW(c)

// 1) 카드 + 디자인 조회 (지연 로딩, 모두 메모리/Redis에서 옴)
cards, _ := u.Cards()
batter := u.Catalog().BatData().Get(100007)

// 2) 카드 강화 (메모리에서 수정 + UPDATE 큐잉)
cards[0].Level += 1
uow.Update(u, cards[0], u.ShardDB())

// 3) 응답 생성 후 종료 → dispatch가 자동으로 u.Commit() 호출
//    실제 DB UPDATE + Redis HSET이 여기서 일어남
```

### Container vs UoW vs Store — 역할 구분

```
                 ┌─────────────────────────────────────────────┐
  서버 시작 시 ──▶│              Container (전역)               │
                 │  - GameDB, ShardDB (database.Database)      │
                 │  - UserCache, UserLock (cache.*)            │
                 │  - DesignStore, DesignSyncer                │
                 │   (모든 요청이 공유. main.go에서 1회 생성)    │
                 └─────────────────────────────────────────────┘
                                     │
                                     │ 요청마다 1개씩
                                     ▼
                 ┌─────────────────────────────────────────────┐
       요청 ────▶│            UnitOfWork (요청 단위)            │
                 │  - userID                                   │
                 │  - catalog (이번 요청의 디자인 카탈로그)        │
                 │  - store map[field]any (로딩된 모델 캐시)     │
                 │  - ops []dbOp (쓰기 큐)                     │
                 │  - 내부에서 Container 참조                   │
                 └─────────────────────────────────────────────┘
                                     │
                                     │ 핸들러/서비스가 사용
                                     ▼
                          데이터 흐름은 아래 다이어그램 참조
```

| 객체 | 수명 | 공유 | 책임 |
|------|------|------|------|
| **Container** | 서버 전체 | 모든 요청이 공유 | 글로벌 자원 보유 (DB 커넥션 풀, Redis 클라이언트, 디자인 Store) |
| **UnitOfWork** | 1 요청 | 요청 1개 전용 | 그 요청의 read 캐시 + write 큐 |
| **Store** (`design.Store`) | 서버 전체 | 모든 요청이 공유 | client_version → Catalog 매핑 (디자인 데이터 라우팅) |
| **Catalog** (`design.Catalog`) | reload까지 | 같은 server_version 요청들이 공유 | 한 server_version의 모든 디자인 데이터 묶음 |

### 데이터 흐름 다이어그램

```
                          ┌──────────────────┐
                          │   HTTP Request   │
                          └────────┬─────────┘
                                   │
                                   ▼
        ┌────────────────────────────────────────────────────┐
        │ dispatch.go                                        │
        │   1. envelope decode                               │
        │   2. catalog := DesignStore.GetByClientVersion()   │  ◀── Container.DesignStore
        │   3. UserLock.Acquire(userID)                      │  ◀── Container.UserLock
        │   4. u := uow.New(Container, userID, catalog)      │
        │   5. handler 호출                                  │
        └────────────────────┬───────────────────────────────┘
                             │
                             ▼
        ┌────────────────────────────────────────────────────┐
        │ Handler / Service                                  │
        │   u.Account() ──┐                                  │
        │   u.Cards()  ───┤  (지연 로딩)                      │
        │   u.Catalog().BatData().Get(...) (디자인 조회)       │
        │   uow.Update(u, card, u.ShardDB())                 │
        │   uow.Create(u, ..., asset, ...)  (UPDATE/INSERT 큐잉)│
        └────────────────────┬───────────────────────────────┘
                             │
                             ▼
        ┌────────────────────────────────────────────────────┐
        │ UoW 내부                                           │
        │                                                    │
        │   ┌─ Read 흐름 ────────────────────────────┐        │
        │   │  store ──hit?──▶ 반환                  │        │
        │   │    │ miss                              │        │
        │   │    ▼                                   │        │
        │   │  UserCache(Redis) ──hit?──▶ store저장+반환 │     │
        │   │    │ miss                              │        │
        │   │    ▼                                   │        │
        │   │  DB(SELECT) ──▶ Redis저장+store저장+반환  │       │
        │   └────────────────────────────────────────┘        │
        │                                                    │
        │   ┌─ Write 흐름 (지연 실행) ─────────────────┐       │
        │   │  store에 즉시 반영                      │       │
        │   │  + ops에 함수 큐잉                      │       │
        │   └────────────────────────────────────────┘       │
        └────────────────────┬───────────────────────────────┘
                             │ 핸들러 종료
                             ▼
        ┌────────────────────────────────────────────────────┐
        │ dispatch가 u.Commit() 자동 호출                    │
        │                                                    │
        │   1) ops를 DB별 그룹핑 → 그룹별 Transaction 실행     │
        │   2) 성공 시 UserCache.SetMulti(store) (Redis)     │
        │   3) 실패 시 UserCache.DeleteAll(userID)           │
        │                                                    │
        │   ┌──────────┐         ┌──────────┐                │
        │   │ GameDB   │         │ ShardDB  │                │
        │   │ (TX 1)   │         │ (TX 2)   │                │
        │   └──────────┘         └──────────┘                │
        │        │                    │                      │
        │        └─────────┬──────────┘                      │
        │                  ▼                                 │
        │          ┌─────────────┐                           │
        │          │ Redis Hash  │  (user:{id} 필드별 갱신)   │
        │          └─────────────┘                           │
        └────────────────────┬───────────────────────────────┘
                             │
                             ▼
                          ┌─────────────┐
                          │  HTTP Resp  │
                          └─────────────┘
```

### 캐시 필드 버전 컨벤션

`internal/uow/field.go`의 상수에 버전 접미사를 박아둔다.

```go
const (
    FieldAccount = "account.v1"
    FieldAssets  = "assets.v1"
    FieldCards   = "cards.v1"
)
```

**스키마 변경 시** (예: `model.Card`에 컬럼 추가):
```go
FieldCards = "cards.v2"   // ← 한 줄 변경
```
배포 즉시 모든 서버가 새 키로 조회 → cache miss → DB 재로드 → 새 키로 저장. 옛 키는 30분 TTL로 자연 소멸. 운영 조작 불필요.

### 주의 사항

- **고루틴 안전 X**: UoW는 요청 1개 전용. 별도 고루틴으로 동시 사용 금지.
- **부분 커밋 가능성**: 여러 DB(GameDB + ShardDB)에 ops가 흩어진 경우 DB별 트랜잭션이라 첫 DB 커밋 후 두 번째 실패 시 부분 반영. 정합성이 중요한 작업은 단일 DB로 정리하거나 멱등성 키 도입(별도 정책 결정 필요).
- **PK 시점**: `Create` (큐잉)는 `Commit()` 후에야 PK가 채워진다. 후속 로직에서 PK가 필요하면 `CreateNow`(즉시 INSERT)를 사용.

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

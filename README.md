# Golang Game Server

Go 기반 게임 서버. Echo + MySQL (Shard) + Redis 다중 인스턴스 + CDN 기반 디자인 데이터.

## 빌드 & 실행

```bash
make build         # 서버 바이너리 빌드
make run           # 서버 실행 (pending 마이그레이션 있으면 경고만, auto-apply 안 함)
make proto         # protobuf 코드 재생성
make vet           # 정적 분석
make client        # 인터랙티브 테스트 클라이언트
make design        # Excel → JSON 변환 (서버용 / TARGET=client 가능)
make design-struct # Excel → Go struct (서버 전용)
make clean         # 빌드 산출물 삭제
```

DB 마이그레이션 ([상세](#db-마이그레이션)):

```bash
make mig-status                                      # 현재 적용 상태
make mig-up                                          # pending 모두 적용
make mig-new-game version=2.01.02 name=add_col_x     # 새 마이그레이션 파일 생성
make mig-baseline                                    # (최초 1회) 기존 DB 를 goose 관리로 편입
```

## 디렉토리 구조

```
.
├── main.go                  # 서버 엔트리포인트
├── config/                  # YAML 설정 파싱
├── router/                  # 디스패치 라우트 + Admin API
├── proto/                   # .proto 원본
├── pb/                      # protoc 생성 코드 (수정 금지)
├── cmd/
│   ├── client/              # 인터랙티브 테스트 클라이언트
│   └── migrator/            # DB 마이그레이션 CLI (goose wrapper)
├── sql/
│   └── migrations/          # 마이그레이션 SQL + goose wrapper (migrator.go) — embed.FS
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
  uint32 action         = 1;
  uint64 user_id        = 2;
  int64  timestamp      = 3;
  string client_version = 4;   // → 서버에서 server_version으로 매핑
  string session_token  = 5;   // Login 시 발급, 이후 envelope에 자동 첨부 (8시간 sliding)
  bytes  body           = 6;
}

message GameResponse {
  uint32   action    = 1;
  int32    code      = 2;       // 200/4xx/5xx + 도메인 코드
  int64    timestamp = 3;
  bytes    body      = 4;
  SyncData sync      = 5;       // 이번 요청에서 변경(생성/수정)된 엔티티 자동 첨부
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

모든 read/write 함수는 `EntityKind` 인자 하나로 owner(user/club) 와 DB(Game/Shard) 라우팅이 자동 결정된다.

| 메서드 | 동작 |
|--------|------|
| `LoadOne[T](u, entity)` | 단일 엔티티 지연 로딩: store → Redis → DB |
| `LoadList[T](u, entity)` | 슬라이스 지연 로딩 |
| `Create[T](u, entity, m)` | INSERT 큐잉. PK는 `Commit()` 후 반영 |
| `CreateNow[T](u, entity, m)` | 즉시 INSERT (PK가 바로 필요할 때, 예: 신규 계정 생성) |
| `Update[T](u, entity, m, cols...)` | UPDATE 큐잉 |
| `u.Commit()` | 모든 ops를 DB별 트랜잭션 실행 + 캐시 반영 |
| `u.Dirty()` | 이번 요청에서 변경된 모델 (dispatch가 envelope sync로 자동 첨부) |
| `u.Account()` / `u.Cards()` / `u.Assets()` / `u.Items()` | 도메인별 편의 래퍼 (LoadOne/LoadList 호출) |
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

`internal/uow/entity_kind.go` 의 `EntityKind` 값에 버전 접미사를 박아둔다. `Name` 이 store/캐시 키, `Owner` 가 scope (user/club), `IsGameDB` 가 라우팅 DB.

```go
var (
    EntityAccount = EntityKind{Name: "account.v1", Owner: OwnerUser, IsGameDB: true}
    EntityAssets  = EntityKind{Name: "assets.v1",  Owner: OwnerUser, IsGameDB: false}
    EntityCards   = EntityKind{Name: "cards.v1",   Owner: OwnerUser, IsGameDB: false}
    EntityItems   = EntityKind{Name: "items.v1",   Owner: OwnerUser, IsGameDB: false}

    EntityClubInfo    = EntityKind{Name: "info.v1",    Owner: OwnerClub, IsGameDB: true}
    EntityClubMembers = EntityKind{Name: "members.v1", Owner: OwnerClub, IsGameDB: true}
)
```

**스키마 변경 시** (예: `model.Card` 에 컬럼 추가):
```go
EntityCards = EntityKind{Name: "cards.v2", Owner: OwnerUser, IsGameDB: false}   // ← Name 만 변경
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

예: `Item` 도메인 (TB_ITEM)

### 체크리스트 (의존성 순서)

| # | 파일 | 작업 |
|---|---|---|
| 0 | `sql/migrations/<db>/<버전>/...sql` | `make mig-new-{game,shard} version=X.YY.ZZ name=add_TB_ITEM` 으로 생성 후 CREATE TABLE 작성. `make mig-up` 으로 적용. ([상세](#db-마이그레이션)) |
| 1 | `proto/item.proto` | 메시지 정의 |
| 2 | `proto/common.proto` | sync 첨부 도메인이면 `SyncData` 에 추가 |
| 3 | — | `make proto` |
| 4 | `internal/model/item.go` | struct + Model 4 메서드 |
| 5 | `internal/uow/entity_kind.go` | `EntityItems` 한 줄 |
| 6 | `internal/uow/wrappers.go` | (선택) `u.Items()` |
| 7 | `internal/errcode/errcode.go` | (선택) 도메인 에러 코드 |
| 8 | `internal/service/item_service.go` | 비즈니스 로직 (uow + errcode 사용) |
| 9 | `internal/handler/action.go` | `ActionGetItems` 등 액션 ID |
| 10 | `internal/handler/item/setup.go` | `itemHandler` struct 정의 + 의존성 와이어업 + `init()` + 등록 |
| 11 | `internal/handler/item/<action>.go` | `(h *itemHandler) GetX(...)` 핸들러 메서드 + `toItemData` |
| 12 | `router/router.go` | blank import (10 의 `init()` 트리거) |

> `repository` 는 일반 CRUD 면 불필요. 특수 SELECT(채널 UID 조회 등) 있을 때만 추가.
>
> setup.go 가 *타입 정의자* (itemHandler struct + svc/c 의존성), action 파일이 *그 타입 위의 메서드*. 같은 패키지라 컴파일은 어느 순서든 통과하지만, **개발 흐름**상 타입과 의존성을 먼저 잡은 뒤 핸들러 메서드를 채우는 게 자연스럽다.

### setup.go (핵심 3 등록)

```go
func setupItemHandler(c *container.Container) {
    h := &itemHandler{svc: service.NewItemService(), c: c}

    handler.RegisterAction(handler.ActionGetItems, h.GetItems,
        func() proto.Message { return &pb.GetItemsRequest{} })

    uow.RegisterModelEntity[*model.Item](uow.EntityItems)          // 변경 추적
    handler.RegisterSyncBuilder(uow.EntityItems, buildItemSync)    // 자동 sync 첨부
}
```

### 자동화 효과

| 등록 한 줄 | 자동으로 따라오는 것 |
|---|---|
| `EntityKind` | LoadOne/LoadList/Create/Update 의 owner·DB 라우팅 |
| `RegisterModelEntity` | UoW 가 변경된 모델 추적 (Create/Update 만 호출하면 됨) |
| `RegisterSyncBuilder` | dispatch 가 응답 envelope `sync` 필드 자동 첨부 |

→ 기존 핸들러/서비스 코드 변경 0.

### 시간 필드 컨벤션

| 종류 | 타입 | 비고 |
|---|---|---|
| DB 가 채우는 timestamp | `time.Time` | DATETIME. `InsertTime`/`UpdateTime` |
| 애플리케이션이 채우는 sentinel 시간 | `int64` | Unix sec. `0` = "없음" 의미 |

서비스에서 `InsertTime: time.Now()` 처럼 *명시* 채워야 캐시와 일치 (DB 자동 채움은 캐시 값과 어긋남).

---

## DB 마이그레이션

**도구**: [pressly/goose v3](https://github.com/pressly/goose) 를 wrap 한 자체 CLI (`cmd/migrator`).
**전략**: 항상 명시적 (`make mig-up`) — auto-migrate 안 함. `make run` 은 pending 이 있으면 경고만 띄움.

### 디렉토리 구조

```
sql/
└── migrations/
    ├── migrations.go                                 # embed.FS 선언
    ├── game/                                         # 게임 DB 용 (TB_ACCOUNT, TB_VERSION 등)
    │   ├── <시간>_init_initial_schema.sql            # baseline (루트, 버전 밖)
    │   ├── 2.01.02/                                  # 릴리즈 단위 묶음
    │   │   └── <시간>_<작업자>_<코멘트>.sql
    │   └── 2.01.03/...
    └── shard/                                        # 샤드 DB 용 (TB_CARD, TB_ITEM 등)
        ├── <시간>_init_initial_schema.sql
        └── 2.01.02/
            └── ...
```

**파일명 규칙**: `<시간>_<작업자>_<코멘트>.sql`
- `<시간>`: `YYYYMMDDhhmmss` (UTC) — goose 의 `version_id`
- `<작업자>`: `git config user.email` 의 `@` 앞부분 (자동 추출), `--author` 로 override 가능
- `<코멘트>`: snake_case 동작 설명

**적용 순서**: 루트 init 파일 (timestamp 순) → 버전 디렉토리들 (lexicographic 순, `2.01.02 < 2.02.00 < 2.10.00`) → 디렉토리 내부 (timestamp 순).
버전 디렉토리 간 timestamp 역전을 허용하기 위해 `goose.WithAllowMissing` 사용.

### 워크플로 (A 개발자 → B 개발자)

**A 개발자: 컬럼 추가**

```bash
# 1. 마이그레이션 파일 생성
make mig-new-game version=2.01.02 name=add_account_last_seen
# → sql/migrations/game/2.01.02/20260520143012_mega_add_account_last_seen.sql

# 2. 파일 편집
vi sql/migrations/game/2.01.02/*_add_account_last_seen.sql
```

```sql
-- +goose Up
ALTER TABLE TB_ACCOUNT ADD COLUMN last_seen BIGINT UNSIGNED NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE TB_ACCOUNT DROP COLUMN last_seen;
```

```bash
# 3. Go 모델에 필드 추가 → 로컬 적용
vi internal/model/account.go
make mig-up
make run                                              # 동작 확인

# 4. 커밋 & 푸시
git add sql/migrations/ internal/model/account.go
git commit -m "feat: TB_ACCOUNT.last_seen 컬럼 추가"
git push
```

**B 개발자: pull 후**

```bash
git pull
make run                                              # ⚠ pending 있음. make mig-up 먼저 실행하세요 — 경고 표시
make mig-up                                           # 적용
make run                                              # 정상 부팅
```

### 명령 reference

| Make 타겟 | 동작 |
|---|---|
| `make mig-new-game version=X.YY.ZZ name=<코멘트>` | 게임 DB 마이그레이션 파일 생성 |
| `make mig-new-shard version=X.YY.ZZ name=<코멘트>` | 샤드 DB 마이그레이션 파일 생성 |
| `make mig-up` | game + 모든 shard 의 pending 적용 |
| `make mig-up-game` / `make mig-up-shard` | 카테고리만 적용 |
| `make mig-up-one` | pending 중 1 개만 적용 (디버깅) |
| `make mig-down` | 직전 적용된 마이그레이션 1 개 롤백 (로컬만 권장) |
| `make mig-status` | 현재 적용 상태 출력 |
| `make mig-baseline` | (최초 1 회) 기존 DB 를 실행 없이 적용 완료로 마킹 |

### 최초 1 회: Baseline

이미 테이블이 존재하는 dev DB 를 goose 관리 체계로 편입할 때:

```bash
make mig-baseline      # SQL 실행 없이 goose_db_version 테이블에만 "적용 완료" 마킹
make mig-status        # 모두 ✓ 인지 확인
```

빈 DB 라면 `make mig-baseline` 대신 그냥 `make mig-up` — init 부터 순서대로 실행됨.

### 컨벤션 / 함정

#### ✅ 권장

**1. NOT NULL 컬럼 추가 시 DEFAULT 도 함께**

```sql
-- ✅ 좋음
ALTER TABLE TB_ACCOUNT ADD COLUMN last_seen BIGINT UNSIGNED NOT NULL DEFAULT 0;

-- ❌ 기존 행 채울 값이 없어 ALTER 자체가 실패
ALTER TABLE TB_ACCOUNT ADD COLUMN last_seen BIGINT UNSIGNED NOT NULL;
```

DEFAULT 가 있으면 기존 모든 행이 그 값으로 자동 채워짐. NULL 허용이 의도가 아니라면 항상 DEFAULT 와 묶어서 작성.

**2. 한 파일 = 한 의도**

테이블 생성 + 컬럼 추가 + 인덱스 + 데이터 변경을 한 파일에 몰아넣지 말고 분리.

```
20260520143012_mega_add_TB_INVENTORY.sql        ← 테이블 생성
20260520143200_mega_idx_inventory_user_id.sql   ← 인덱스
20260520143500_mega_backfill_inventory.sql      ← 초기 데이터
```

이유: 한 파일이 중간에 실패하면 그 파일만 롤백되고 다음 파일은 시도 안 됨. 작은 단위로 쪼개야 어느 단계에서 멈췄는지 파악과 재시도가 쉽다.

**3. 데이터 백필은 별도 작업으로 분리**

마이그레이션 파일에는 DDL + 가벼운 DML 만. 무거운 백필(수십만 행 UPDATE 등) 은:
- 별도 admin endpoint 로 노출하거나
- one-shot cron 잡으로 처리

이유:
- 마이그레이션은 부팅 차단 단계가 아니지만 `make mig-up` 의 시간을 늘려 dev 흐름을 방해
- 트랜잭션 한 번에 거대한 UPDATE 가 들어가면 락 시간 증가
- 실패 시 부분 재시도가 어려움 (마이그레이션은 "한 번에 전부 또는 전무")

**4. Down 마이그레이션 작성 — 로컬 검증용**

```sql
-- +goose Up
ALTER TABLE TB_ACCOUNT ADD COLUMN last_seen BIGINT UNSIGNED NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE TB_ACCOUNT DROP COLUMN last_seen;
```

로컬에서 `make mig-down` 으로 되돌렸다가 다시 `mig-up` 으로 적용해보며 양방향 검증 가능. **프로덕션에서는 호출하지 않는 게 컨벤션** — 잘못된 마이그레이션은 forward fix 로 해결 (아래 ❌ 4 참고).

복구 불가한 경우 (예: DROP COLUMN 한 데이터를 되살릴 수 없음) 는 명시:
```sql
-- +goose Down
SELECT 'no down: data loss not recoverable' AS warning;
```

**5. 멀티-statement 블록은 `StatementBegin/End` 로 묶기**

goose 는 기본적으로 `;` 로 SQL 을 분리. 한 덩어리로 실행해야 하는 경우 명시:

```sql
-- +goose Up
-- +goose StatementBegin
INSERT INTO TB_VERSION (client_version, server_version, app_id, ...)
VALUES ('2.01.02', '2.01.02.00', 'com.example.app', ...),
       ('2.01.02', '2.01.02.00', 'com.example.app2', ...);
-- +goose StatementEnd
```

또는 스토어드 프로시저 / 트리거 정의처럼 `;` 가 본문에 들어가는 경우 필수.

---

#### ❌ 금지 / 함정

**1. 이미 머지된 마이그레이션 파일 수정**

goose 는 `goose_db_version` 테이블에 **`version_id` 만** 저장. 파일 내용은 기록 안 함. 그래서:

- A 가 작성한 파일이 A 의 로컬 DB 에 적용됨 → `version_id` 가 테이블에 박힘
- A 가 파일 내용을 고치고 PR 머지
- B 가 pull 받음 → goose 는 동일 `version_id` 가 이미 적용됐다고 판단해 **수정된 SQL 을 실행하지 않음**
- B 의 DB 는 A 와 다른 상태로 끝남

수정이 필요하면 **새 마이그레이션 파일** 로 fix 를 추가. 머지된 파일은 불변 원칙.

**2. 마이그레이션 파일명 변경**

`version_id` 는 파일명 앞 14 자리 timestamp 에서 추출됨. 파일명을 바꾸면 `version_id` 가 바뀜:
- 누군가의 DB 에는 옛 `version_id` 가 적용된 채로 남음
- 새 `version_id` 는 미적용으로 보여 재실행 → `CREATE TABLE` 같은 비-멱등 DDL 은 충돌

`<작업자>` 부분이나 `<코멘트>` 만 바꿔도 timestamp 가 그대로면 OK. **timestamp 자체는 절대 변경 X**.

**3. `goose_db_version` 테이블 수동 편집**

DB 가 진짜 망가졌을 때의 최후 수단. 정상 상황에서는 절대 손대지 말 것. 한 번 박힌 적용 이력을 수동으로 지우면:
- `mig-up` 이 이미 실행된 SQL 을 다시 실행 → 비-멱등 DDL 실패
- 다른 개발자 환경과 상태 불일치

복구가 필요하다면 forward fix migration 으로 해결.

**4. 프로덕션에서 `make mig-down` 호출**

Down 은 로컬 dev 검증용. 프로덕션 forward-only 정책:
- Down 마이그레이션은 작성/테스트했지만 실제로 안 돌아본 경우 많음
- 이미 새 코드가 새 스키마에 의존해 배포된 상태일 가능성
- DROP COLUMN 류는 데이터 손실

잘못된 마이그레이션은 **새 마이그레이션을 추가해 원하는 상태로 forward fix**:

```sql
-- 잘못 추가한 컬럼을 되돌리고 싶다면
-- +goose Up
ALTER TABLE TB_ACCOUNT DROP COLUMN wrong_col;

-- +goose Down
-- (의도적으로 비움 — 이건 fix migration)
```

**5. 큰 테이블에 락을 길게 거는 ALTER**

MySQL 의 일부 DDL 은 테이블을 락. 행이 수백만 이상인 테이블에 무심코 ALTER 하면 서비스 영향. 대안:
- InnoDB online DDL 옵션 명시: `ALGORITHM=INPLACE, LOCK=NONE`
  ```sql
  ALTER TABLE TB_CARD ADD COLUMN new_col INT NOT NULL DEFAULT 0,
    ALGORITHM=INPLACE, LOCK=NONE;
  ```
- 큰 테이블이 예상되면 `pt-online-schema-change` 같은 도구를 고려 (마이그레이션 외부)

dev 환경에서는 문제가 안 보이므로 **운영 테이블 규모를 항상 인지** 하고 작성.

**6. MySQL DDL 의 implicit commit**

```sql
-- +goose Up
START TRANSACTION;
ALTER TABLE TB_X ADD COLUMN a INT;   -- 여기서 implicit commit 발생
ALTER TABLE TB_X ADD COLUMN b INT;   -- 위 ALTER 가 이미 commit 됐음
ROLLBACK;                            -- ALTER 들은 롤백 안 됨
```

MySQL 의 DDL 은 트랜잭션 안에 있어도 implicit commit 됨. 그래서:
- 한 마이그레이션에 여러 ALTER 를 넣고 중간 실패해도 앞쪽 ALTER 는 commit 된 채로 남음
- → **컨벤션 #2 (한 파일 = 한 의도)** 가 더 중요해지는 이유

**7. 멀티-인스턴스 동시 부팅 시 마이그레이션 경쟁**

이 프로젝트는 `auto_migrate` 안 함 (배포 파이프라인에서 명시적 `mig-up`) 이라 해당 없음. 다만 만약 부팅 시 자동 적용을 켠다면:
- 여러 인스턴스가 동시에 `mig-up` 시도 → goose 가 advisory lock 사용 안 함 → 같은 마이그레이션이 두 번 실행될 수 있음
- 보호하려면: 한 인스턴스만 실행하는 정책, 또는 외부 락 (Redis 락 등) 으로 직렬화

**8. 같은 timestamp 충돌 (PR 동시 작성)**

A 와 B 가 정확히 같은 초에 `make mig-new-game` 호출하면 동일 `version_id` 파일이 만들어짐. 머지 시점에 충돌하면:
- PR 리뷰에서 한쪽 파일명을 1 초 늦춰 rename (단, **머지 전** 에 한해)
- 머지 후 발견되면 forward fix 로 처리

타임스탬프는 초 단위라 사실상 거의 발생 X. 발생하면 goose 가 PK 충돌 (`goose_db_version.id` 가 아니라 version_id 의 중복 사용) 로 실패하므로 늦게 감지될 일은 없음.

### 동시 작업 (A 와 B 가 같은 날 마이그레이션 작성)

- 파일명 = timestamp 라 충돌 X. 머지 순서대로 version_id 가 결정됨.
- 같은 컬럼을 양쪽이 건드리면 두 번째 ALTER 가 실패 → PR 리뷰에서 잡아야 함 (도구가 막아주지 않음).

### Go 측 통합

```
sql/migrations/
├── migrations.go      # package migrations — //go:embed game shard (embed.FS)
├── migrator.go        # package migrations — MigrateUp/Down/UpByOne/Status/Baseline
├── game/
└── shard/

cmd/migrator/main.go   # 위 함수를 CLI 서브커맨드 (create / up / down / status / baseline) 로 노출
```

- 마이그레이션 SQL 과 그것을 적용하는 Go wrapper 가 **같은 패키지(`sql/migrations`)** 에 공존 — `internal/admin_ui` 가 `embed.FS` 와 `FS()` 헬퍼를 한 패키지에 두는 것과 동일한 패턴.
- 적용 순서는 `migrator.go` 의 walker 가 결정: **루트 init 파일 → 버전 디렉토리들** (lexicographic).
- **부팅 시 자동 적용 안 함** — `main.go` 는 마이그레이션을 호출하지 않음. `make run` 의 pre-check 가 pending 을 경고로만 표시.

---

## Admin Web UI

운영자가 시간 점프·버전 추가/삭제 같은 관리 작업을 브라우저에서 할 수 있는 페이지. **Go 바이너리 안에 React SPA 가 embed** 되어 한 프로세스로 배포된다.

접속: `http://localhost:8089/admin/ui/`

### 기술 스택

| 영역 | 도구 | 역할 |
|---|---|---|
| UI 라이브러리 | **React 18** | 컴포넌트 기반 UI |
| 언어 | **TypeScript** | 타입 안전 + IDE 자동완성 |
| 빌드/dev 서버 | **Vite** | 빠른 dev (HMR) + 운영 번들 빌드 |
| 스타일링 | **Tailwind CSS v4** | utility-first CSS, `<div className="rounded bg-slate-900 ...">` 형태로 디자인 |
| 클라이언트 라우팅 | **react-router-dom** | `/clock`, `/version` 같은 페이지 전환을 *서버 요청 없이* 처리 |
| 날짜 선택 | **react-flatpickr** | 24시간제 + 초 단위 datepicker |
| 정적 파일 임베드 | **Go 1.22 `//go:embed`** | 빌드 산출물(`dist/`) 을 Go 바이너리에 포함 |

### 디렉토리

```
web/admin/                       # React 프로젝트 (npm 영역)
  package.json
  vite.config.ts                 # outDir = ../../internal/admin_ui/dist, dev proxy 8089
  index.html
  src/
    main.tsx                     # 엔트리. BrowserRouter basename="/admin/ui"
    App.tsx                      # 좌측 nav + Routes 정의
    index.css                    # @import "tailwindcss"
    api/client.ts                # fetch wrapper (clockApi, versionApi)
    pages/
      ClockPage.tsx              # 시간 이동
      VersionPage.tsx            # 버전 추가/삭제/목록

internal/admin_ui/
  ui.go                          # //go:embed all:dist
  dist/                          # Vite 빌드 산출물 (placeholder index.html 만 git tracked)
```

### 빌드 / 실행

```bash
# 의존성 설치 (최초 1회)
cd web/admin
npm install

# 운영 빌드 (한 바이너리)
cd ..
make release                     # admin-ui + go build 한 번에
./server

# 개발 모드 (HMR)
make run                         # 터미널 1 - Go 서버 (8089)
cd web/admin && npm run dev      # 터미널 2 - Vite (5173)
# 브라우저: http://localhost:5173/  (vite proxy 가 /admin/* 를 8089 로 포워딩)
```

### 동작 흐름

```
[운영 모드]
브라우저 → GET /admin/ui/        → Go 서버 → dist/index.html (embed 에서)
                                            ↓
                                  <script src="...index-XXX.js">
                                            ↓
                                  React 앱 부팅 → BrowserRouter
                                            ↓
                                  /clock 경로 → ClockPage 렌더
                                            ↓
                                  fetch('/admin/clock') 호출
                                  → 같은 origin Go 서버 → JSON 응답

[개발 모드 — HMR]
브라우저 → http://localhost:5173 → Vite dev server (TS 즉시 컴파일)
                                            ↓
                                  소스 수정 시 변경분만 자동 반영 (HMR)
                                            ↓
                                  fetch('/admin/...') → Vite proxy
                                  → http://localhost:8089/admin/...
```

### SPA 라우팅 (직접 URL 접속 처리)

`/admin/ui/clock` 같이 *서버 URL* 로 직접 접속하려면 — 서버는 그 경로의 파일이 없으므로 기본 동작은 404.
**해결**: Go 서버가 알려진 정적 파일(JS/CSS/이미지) 외에는 모두 `index.html` 로 fallback → React Router 가 클라이언트에서 라우팅.

[router/admin.go](router/admin.go) `registerAdminUI` 의 SPA fallback 로직이 그 역할.

### 새 페이지 추가 패턴

1. `web/admin/src/pages/MyPage.tsx` 추가
2. `web/admin/src/App.tsx` 에 두 줄:
   ```tsx
   <NavItem to="/my">내 페이지</NavItem>
   <Route path="/my" element={<MyPage />} />
   ```
3. API 호출이 필요하면 `src/api/client.ts` 에 함수 추가
4. Go 측에 admin 엔드포인트 추가 ([router/admin.go](router/admin.go))
5. `npm run build` (또는 `make release`)

### ⚠️ 인증

현재 `/admin/*` 라우트는 **인증 없음** ([admin.go](router/admin.go) 의 `TODO`). 사내망/VPN 뒤에서만 사용하거나 운영 노출 전 Basic Auth 미들웨어 추가 필요:

```go
g.Use(middleware.BasicAuth(func(u, p string, _ echo.Context) (bool, error) {
    return u == "admin" && p == os.Getenv("ADMIN_PASS"), nil
}))
```

---

## Config (`config.yaml`)

```yaml
server:
  port: "8089"

databases:
  # weight: 신규 유저 자동 할당 가중치. 0 이면 풀에서 제외(시스템 DB).
  - { name: game,    shard_id: 0,  weight: 0,   host: localhost, port: 3306, user: root, password: ..., dbname: N_GAME }
  - { name: shard_1, shard_id: 10, weight: 100, host: localhost, port: 3306, user: root, password: ..., dbname: N_SHARD_10 }

redis:
  - { name: lock,          host: localhost, port: 6379, password: ..., db: 0 }   # UserLock (분산 락)
  - { name: user_session,  host: localhost, port: 6379, password: ..., db: 1 }   # 8시간 sliding 세션 토큰
  - { name: user_cache,    host: localhost, port: 6379, password: ..., db: 2 }   # 유저 도메인 Hash 캐시
  - { name: club_cache,    host: localhost, port: 6379, password: ..., db: 3 }   # 클럽 도메인 Hash 캐시
  - { name: reload_pubsub, host: localhost, port: 6379, password: ..., db: 15 }  # 디자인/리소스 reload Pub/Sub

cdn:
  design_base_url: "https://example.com/design"
  http_timeout_seconds: 10
```

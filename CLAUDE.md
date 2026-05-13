# CLAUDE.md

## Project
Future Next Baseball — Go 야구 게임 서버 (Echo + MySQL 샤딩 + Redis 다중 인스턴스 + CDN 디자인 데이터).

## Stack
- Go 1.25.4 / Echo v4 / sqlx / go-redis v9 / Protobuf
- Config: `config.yaml`

## Layout
```
main.go                       # config → DB/Redis/Design init → Echo
config/                       # YAML 파서
router/                       # /api 디스패처, /admin
proto/, pb/                   # 원본 + 생성 코드
internal/
  app/                        # AppParams (import cycle 회피)
  container/                  # 공유 의존성
  database/                   # sqlx + Model CRUD, 샤드 레지스트리
  cache/                      # Redis 레지스트리, UserCache, UserLock
  model/                      # TB_* 매핑 (Model 인터페이스)
  repository/                 # DB 접근
  service/                    # 비즈니스 로직 (cache-first)
  uow/                        # Unit of Work (제네릭 LoadOne/LoadList/Create)
  handler/                    # 단일 라우트 디스패처 + 도메인 서브패키지
  design/                     # CDN 로더 + Snapshot Store + Syncer (Pub/Sub)
    schema/                   # excel2struct 자동 생성 (수정 금지)
  middleware/                 # logger, recover
tools/
  excel2json/                 # 디자인 xlsx → JSON + manifest
  excel2struct/               # 디자인 xlsx → Go struct
```

## Architecture
```
POST /api  GameRequest{action, user_id, timestamp, client_version, body}
   ↓ dispatch (UserLock 획득 → ctx 세팅)
Handler → Service → UoW (cache → Redis → DB) → Commit
   ↓ GameResponse{action, code, timestamp, body}
```

## Key Patterns

### Single-Route Dispatcher (`internal/handler/dispatch.go`)
- `POST /api`: envelope 디코드 → action 라우팅 → 자동 `CommitOrRollback`
- 컨텍스트: `action_id`, `user_id`, `client_version`, `uow`
- userID≠0 이면 Redis 분산락 (`UserLock`) 자동 획득

### Action 등록
```go
// internal/handler/<domain>/setup.go
func init() { handler.Register(setup) }
func setup(c *handler.Container) { handler.RegisterAction(ActionXxx, h.Handle, &pb.XxxRequest{}) }
```

### UoW (`internal/uow/`)
- `LoadOne[T Model](u, field, db)` / `LoadList[T]` — store→Redis→DB lazy 로드
- `Create[T]` (큐잉) / `CreateNow[T]` (즉시 INSERT, FieldAccount는 userID 자동 할당)
- `Commit()`: ops를 DB별 트랜잭션으로 묶어 실행 → UserCache.SetMulti

### Database (Model 기반)
```go
db.FindOne(&m, "...", args...)
db.FindList(&list, model.X{}, "...", opt, args...)
db.Create(&m); db.Save(&m, cols...); db.Remove(&m, pk)
db.Transaction(func(tx *sqlx.Tx) error { ... })
database.IsNotFound(err)
```
- Model 인터페이스: `TableName()`, `PrimaryKey()`, `SetPrimaryKey(int64)`, `IsSingleton() bool`
- 모든 메서드는 포인터 리시버
- 샤드: `database.GetShard(account.DBShardID)`

### Redis 다중 인스턴스 (`internal/cache/`)
- 레지스트리: `cache.Init(name, ...)`, `cache.Get(name)`, `CloseAll()`
- 이름: `NameUserLock`, `NameUserCache`, `NameDesignSync`
- `UserLock.Acquire/Release` — `SetArgs{Mode:"NX"}` + Lua 해제
- `UserCache` — Hash 구조, `GetOrLoad[T]` 제네릭 cache-aside

### JSON 컬럼 (`model.JSONField[T]`)
- `Scan` (포인터 리시버), `Value`/`MarshalJSON` (값 리시버 — sqlx 호환)

### Design Data (`internal/design/`)
- TB_VERSION의 `is_active=1` 행 → server_version DESC 정렬
- 최신 → `current`, 그 다음 → `previous`
- `client_version → server_version` 매핑은 `versionMap`에 저장
- 핸들러: `handler.Design(c)` / `handler.GetXxxDesign(c, id)` 사용
- 동기화: 단일 Pub/Sub 채널 → 모든 서버 `LoadActive` 재실행
- Admin: `POST /admin/design/reload`

### Error Codes (`internal/handler/error_code.go`)
- 200/400/429/500 + 1xxx(계정) / 2xxx(카드) / 3xxx(자산) / 4xxx(버전)
- `handler.Errorf(code, fmt, ...)` / `BadRequest(...)`

## Conventions
- 시간: `uint32(time.Now().Unix())`
- `db` 태그 = 컬럼명, PK는 INSERT 자동 제외
- `db:"<col>,auto"`: SELECT 매핑은 받되 INSERT/UPDATE 절에서 제외. DB의 `DEFAULT`/`ON UPDATE CURRENT_TIMESTAMP` 자동 관리 컬럼용 (예: `insert_time`, `update_time`)
- Service는 `database` 직접 의존 X (`IsNotFound` 사용)
- 자동 생성 파일(`schema/`, `pb/`) 직접 수정 금지
- 새 도메인: model → repository → service → handler subpkg(`init()` 등록) → action ID

## Build / Run
```bash
make run                 # config.yaml 기준 실행
make build               # 바이너리
make proto               # .proto → pb/
make design              # tools/excel2json
make design-struct       # tools/excel2struct
```

---

## 작업 가이드라인

LLM 코딩 시 흔한 실수를 줄이기 위한 행동 지침. 프로젝트별 지시와 함께 적용한다.

**트레이드오프:** 이 가이드라인은 속도보다 신중함을 우선한다. 사소한 작업에서는 판단에 맡긴다.

### 1. 코딩 전에 먼저 생각하라

**가정하지 말고, 혼란을 숨기지 말고, 트레이드오프를 드러내라.**

구현 전에:
- 가정은 명시적으로 밝힌다. 확신이 없으면 묻는다.
- 해석이 여럿 가능하면 모두 제시한다 — 혼자 고르지 않는다.
- 더 단순한 접근이 있으면 말한다. 필요하면 반대 의견을 낸다.
- 불명확하면 멈춘다. 무엇이 헷갈리는지 짚고, 묻는다.

### 2. 단순함 우선

**문제를 해결하는 최소한의 코드. 투기적인 것은 금지.**

- 요구되지 않은 기능은 추가하지 않는다.
- 일회성 코드에 추상화는 만들지 않는다.
- 요청되지 않은 "유연성"이나 "설정 가능성"은 넣지 않는다.
- 불가능한 시나리오에 대한 에러 처리는 작성하지 않는다.
- 200줄을 썼는데 50줄로 가능하면, 다시 쓴다.

스스로에게 물어라: "시니어 엔지니어가 이걸 보면 과하다고 할까?" — 그렇다면 단순화한다.

### 3. 외과적 변경

**필요한 곳만 건드린다. 자신이 만든 것만 정리한다.**

기존 코드를 수정할 때:
- 인접 코드, 주석, 포매팅을 "개선"하지 않는다.
- 망가지지 않은 것은 리팩토링하지 않는다.
- 기존 스타일을 따른다 — 내 취향과 달라도.
- 관련 없는 죽은 코드를 발견하면 언급만 한다 — 삭제하지 않는다.

변경이 고아(orphan)를 만들었을 때:
- *내 변경*으로 인해 사용되지 않게 된 import/변수/함수만 제거한다.
- 변경 이전부터 있던 죽은 코드는 요청 없으면 건드리지 않는다.

검증: 변경된 모든 줄이 사용자의 요청과 직접 연결되어야 한다.

### 4. 목표 지향 실행

**성공 기준을 정의한다. 검증될 때까지 반복한다.**

작업을 검증 가능한 목표로 바꾼다:
- "검증 추가" → "잘못된 입력에 대한 테스트를 쓰고, 통과시킨다"
- "버그 수정" → "버그를 재현하는 테스트를 쓰고, 통과시킨다"
- "X 리팩토링" → "리팩토링 전후 모두 테스트가 통과하는지 확인한다"

여러 단계 작업이라면 간단한 계획을 먼저 밝힌다:

```
1. [단계] → 검증: [확인 방법]
2. [단계] → 검증: [확인 방법]
3. [단계] → 검증: [확인 방법]
```

강한 성공 기준은 독립적으로 반복(loop)할 수 있게 해준다. "그냥 되게 해" 같은 약한 기준은 매번 추가 설명을 요구한다.

---

**이 가이드라인이 작동하고 있다는 신호:** diff에 불필요한 변경이 줄어들고, 과도한 복잡도로 인한 재작성이 줄어들고, 실수 후의 사후 질문이 아니라 구현 *전의* 명료화 질문이 늘어난다.

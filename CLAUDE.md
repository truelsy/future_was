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

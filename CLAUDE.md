# CLAUDE.md

## Project
Future Next Baseball — Go 기반 야구 게임 서버 (Echo + MySQL + Redis).

## Stack
- **Language**: Go 1.25.4
- **Framework**: Echo v4
- **DB**: MySQL (sqlx, 다중 DB 지원)
- **Cache**: Redis v9 (go-redis)
- **Config**: YAML (`config.yaml`)
- **Protocol**: Protobuf (요청/응답 바디)
- **Port**: 8080

## Structure
```
main.go                          # config 로드 → DB/Redis init → Echo 기동
config/config.go                 # YAML 파서, DSN 생성
router/router.go                 # Container 생성 → handler.SetupAll(/api)
pb/                              # protobuf 생성 코드 (account, common)
proto/                           # .proto 원본
internal/
  container/                     # 공유 의존성 (GameDB, UserCache)
  database/                      # sqlx 래퍼 + Model 기반 CRUD
  cache/                         # Redis 초기화 + UserCache (Hash 기반)
  model/                         # TB_ACCOUNT, TB_ASSET (Model 구현체)
  repository/                    # DB 접근 계층
  service/                       # 비즈니스 로직 (Cache-First)
  handler/                       # init() 자동 등록 패턴, protobuf I/O
```

## Architecture
```
Request → Echo → Handler → Service → UserCache (Hit 시 반환)
                              ↓ Miss
                          Repository → MySQL → Cache Set
```
- Handler: protobuf 바인딩/응답, 입력 검증
- Service: Cache-First. 캐시 조회 → 연산 → DB 반영 → 캐시 갱신
- Repository: `database` 패키지의 Model 기반 CRUD 사용 (생 SQL 지양)
- Model: `TableName()`, `PrimaryKey()` 구현 + `db` 태그로 컬럼 추출 (리플렉션 + 메타 캐시)

## Key Patterns

### Handler 자동 등록
`init()`에서 `handler.Register(fn)` 호출 → `SetupAll(api, container)`에서 일괄 실행.
```go
func init() { Register(registerAccountHandler) }
```

### Database CRUD (Model 기반)
```go
db.FindOne(&account, "channel_uid = ? AND is_active > 0", cuid)
db.FindList(&list, model.Asset{}, "user_id = ?", opt, uid)
db.Create(&model)                           // LastInsertId 반환
db.Save(&model, "device_id", "update_time") // 지정 컬럼만 UPDATE
db.Remove(&model, pkValue)
db.CountOf(model.X{}, "where ...")
db.Transaction(func(tx *sqlx.Tx) error { ... })
// raw: db.RawGet / RawSelect / RawExec
```
에러 판별: `database.IsNotFound(err)`

### UserCache (Redis Hash)
- Key: `user:{user_id}`, Field별 JSON 저장, TTL 30분 (set 시 리셋)
- `cache.GetOrLoad[T](userCache, userID, field, loader)` — 제네릭 cache-aside
- 필드 상수: `cacheFieldAccount = "account"`, `cacheFieldAssets = "assets"`

### Protobuf I/O
- `handler.BindProto(c, &req)` — body → proto
- `handler.SendProto(c, code, msg)` — proto → response (현재 `c.JSON`로 전송, Blob 주석 처리됨)
- `SendBadRequest` / `SendInternalError` / `SendError` — `pb.ErrorResponse`

## Domain Models

### TB_ACCOUNT (`model.Account`, PK: `user_id`)
`user_id`, `channel_uid`, `device_id`, `is_active` (0:비활성/1:정식/2:게스트), `db_shard_id`, `table_id`, `insert_time`, `update_time`

### TB_ASSET (`model.Asset`, PK: `idx`, UNIQUE: `(user_id, asset_id)`)
`idx`, `user_id`, `asset_id`, `quantity` (int64, 음수 가능), `insert_time`, `update_time`

## APIs
- `GET /` — Welcome (proto)
- `GET /health` — Health (proto)
- `POST /api/login` — `pb.LoginRequest{channel_uid, device_id}` → `pb.LoginResponse{user_id, channel_uid, is_new}`. 없으면 신규 계정 생성.

## Services

### AccountService
- `Login(channelUID, deviceID)` — 조회 → 없으면 생성, 캐시 갱신. `(account, isNew, err)`
- `GetAccount(userID)` — cache-aside

### AssetService
- `GetAssets(userID)` — cache-aside 전체 목록
- `AddAsset(userID, assetID, qty)` — 캐시에서 탐색 → 없으면 Create / 있으면 UpdateQuantity → 캐시 갱신
- `ConsumeAsset(userID, assetID, qty)` — 부족 시 에러, 차감 후 DB/캐시 반영

## Conventions
- 시간: `uint32(time.Now().Unix())` (Unix epoch seconds)
- `db` 태그 = 컬럼명, PK는 INSERT 자동 제외 (auto-increment)
- Service는 `database` 패키지 직접 의존하지 않음 (`IsNotFound` 사용)
- 새 도메인 추가 시: model → repository → service → handler (`init()` 등록) 순서

## Config 예시 (`config.yaml`)
```yaml
server: { port: "8080" }
databases:
  - { name: "game", host: "localhost", port: "3306", user: "root", password: "...", dbname: "FUTURE_NPB_GAME" }
redis: { host: "localhost", port: "6379", password: "", db: 0 }
```
DB 인스턴스 접근: `database.Get("game")`

## Build / Run
```bash
go run main.go          # config.yaml 기준 실행
go build -o server .
```

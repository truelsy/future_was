// cmd/migrator 는 DB 마이그레이션 CLI 진입점.
//
// 디렉토리 구조:
//   sql/migrations/<db>/<시간>_init_<...>.sql                       ← baseline (루트)
//   sql/migrations/<db>/<버전>/<시간>_<작업자>_<코멘트>.sql        ← 릴리즈별 변경
//
//   - <db>:  game | shard
//   - <버전>: x.yy.zz (예: 2.01.02) — TB_VERSION 의 client_version 포맷
//   - <시간>: YYYYMMDDhhmmss → goose 의 version_id
//
// 사용 예 (Makefile 래퍼 권장):
//   go run ./cmd/migrator create --db game --version 2.01.02 --name add_account_last_seen
//   go run ./cmd/migrator up
//   go run ./cmd/migrator up --db game
//   go run ./cmd/migrator down
//   go run ./cmd/migrator status
//   go run ./cmd/migrator baseline                   # 기존 dev DB 를 goose 관리로 편입
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"future_was/config"
	"future_was/internal/database"
	"future_was/sql/migrations"
)

// 마이그레이션이 적용될 카테고리 단위. flag --db 값과 매핑.
type targetSet struct {
	game  bool
	shard bool
}

func (t targetSet) any() bool { return t.game || t.shard }

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "create":
		runCreate(args)
	case "up":
		runUp(args)
	case "up-by-one":
		runUpByOne(args)
	case "down":
		runDown(args)
	case "status":
		runStatus(args)
	case "baseline":
		runBaseline(args)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`Usage: migrator <command> [flags]

Commands:
  create    새 마이그레이션 파일 생성 (--db, --version, --name 필수)
  up        pending 마이그레이션 모두 적용 (--db game|shard|all, 기본 all)
  up-by-one pending 마이그레이션 중 1 개만 적용 (디버깅)
  down      직전 적용된 마이그레이션 1 개 롤백 (--db, 1 회 호출 단위)
  status    현재 적용 상태 출력 (--db)
  baseline  기존 dev DB 의 모든 마이그레이션을 실행 없이 적용 완료로 마킹

Flags:
  --db        대상 카테고리: game | shard | all (기본 all)
  --version   create 시 사용할 버전 디렉토리 (예: 2.01.02)
  --name      create 시 사용할 마이그레이션 이름 (snake_case)
  --author    작업자 (생략 시 git config user.email 의 로컬 파트, 그 다음 $USER)
  --config    config.yaml 경로 (기본 ./config.yaml)`)
}

// --------------------------------------------------------------------------
// create
// --------------------------------------------------------------------------

var versionRE = regexp.MustCompile(`^\d+\.\d{2}\.\d{2}$`)
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)

func runCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	dbFlag := fs.String("db", "", "game | shard")
	version := fs.String("version", "", "버전 디렉토리 (x.yy.zz)")
	name := fs.String("name", "", "마이그레이션 이름 (snake_case)")
	authorFlag := fs.String("author", "", "작업자 (생략 시 자동)")
	_ = fs.Parse(args)

	if *dbFlag != "game" && *dbFlag != "shard" {
		fail("--db game|shard 필수")
	}
	if !versionRE.MatchString(*version) {
		fail("--version 형식 오류 (x.yy.zz, 예: 2.01.02): %q", *version)
	}
	if !nameRE.MatchString(*name) {
		fail("--name 은 snake_case 만 허용 (예: add_account_last_seen): %q", *name)
	}

	author, err := resolveAuthor(*authorFlag)
	if err != nil {
		fail("작업자 확정 실패: %v", err)
	}

	ts := time.Now().UTC().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s_%s.sql", ts, author, *name)
	dir := filepath.Join("sql", "migrations", *dbFlag, *version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail("디렉토리 생성: %v", err)
	}
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err == nil {
		fail("이미 존재: %s", path)
	}

	template := fmt.Sprintf(`-- migration: %s
-- author:    %s
-- created:   %s UTC

-- +goose Up


-- +goose Down

`, *name, author, time.Now().UTC().Format(time.RFC3339))

	if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
		fail("파일 작성: %v", err)
	}
	fmt.Printf("✓ created: %s\n", path)
}

// resolveAuthor 작업자명 결정.
//  1. --author 플래그
//  2. git config user.email 의 로컬 파트 (mega@com2us.com → mega)
//  3. $USER 환경변수
// 결과는 영문 소문자 + 숫자 + '_' 만 남기고 sanitize.
func resolveAuthor(flag string) (string, error) {
	if flag != "" {
		return sanitizeAuthor(flag), nil
	}
	// git config
	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		email := strings.TrimSpace(string(out))
		if i := strings.Index(email, "@"); i > 0 {
			email = email[:i]
		}
		if s := sanitizeAuthor(email); s != "" {
			return s, nil
		}
	}
	// $USER
	if u := os.Getenv("USER"); u != "" {
		if s := sanitizeAuthor(u); s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("--author 명시하거나 git config user.email / $USER 를 설정")
}

func sanitizeAuthor(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune('_')
		}
	}
	return b.String()
}

// --------------------------------------------------------------------------
// up / up-by-one / down / status / baseline
// --------------------------------------------------------------------------

func runUp(args []string)        { withDBs(args, "up", upHandler) }
func runUpByOne(args []string)   { withDBs(args, "up-by-one", upByOneHandler) }
func runDown(args []string)      { withDBs(args, "down", downHandler) }
func runStatus(args []string)    { withDBs(args, "status", statusHandler) }
func runBaseline(args []string)  { withDBs(args, "baseline", baselineHandler) }

type dbTarget struct {
	label string                 // 표시용 (예: "GameDB[N_GAME]" 또는 "ShardDB[N_SHARD_10]")
	cat   migrations.MigrationCategory
	cfg   config.DBConfig
}

// withDBs 공통: config.yaml → DB 연결 → 각 DB 에 대해 handler 호출.
func withDBs(args []string, cmd string, handler func(ctx context.Context, db *sql.DB, t dbTarget) error) {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dbFlag := fs.String("db", "all", "game | shard | all")
	cfgPath := fs.String("config", "config.yaml", "config.yaml 경로")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fail("config load: %v", err)
	}

	tg := targetsFromFlag(*dbFlag)
	if !tg.any() {
		fail("--db 는 game | shard | all 중 하나")
	}

	targets, err := dbTargetsFor(cfg, tg)
	if err != nil {
		fail("DB 대상 결정 실패: %v", err)
	}
	if len(targets) == 0 {
		fmt.Println("⚠ 대상 DB 없음 (config.yaml 의 databases 가 비어있거나 필터에 매칭 안 됨)")
		return
	}

	ctx := context.Background()
	for _, t := range targets {
		db, err := sql.Open("mysql", t.cfg.DSN())
		if err != nil {
			fail("[%s] open: %v", t.label, err)
		}
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			fail("[%s] ping: %v", t.label, err)
		}
		if err := handler(ctx, db, t); err != nil {
			_ = db.Close()
			fail("[%s] %s: %v", t.label, cmd, err)
		}
		_ = db.Close()
	}
}

func upHandler(ctx context.Context, db *sql.DB, t dbTarget) error {
	fmt.Printf("=== %s ===\n", t.label)
	before, err := migrations.CountPending(ctx, db, t.cat)
	if err != nil {
		return err
	}
	if before == 0 {
		fmt.Printf("✓ no pending migrations\n")
		return nil
	}
	if _, err := migrations.MigrateUp(ctx, db, t.cat); err != nil {
		return err
	}
	after, err := migrations.CountPending(ctx, db, t.cat)
	if err != nil {
		return err
	}
	fmt.Printf("✓ applied %d migration(s)  (%d → %d pending)\n", before-after, before, after)
	return nil
}

func upByOneHandler(ctx context.Context, db *sql.DB, t dbTarget) error {
	fmt.Printf("=== %s ===\n", t.label)
	if err := migrations.MigrateUpByOne(ctx, db, t.cat); err != nil {
		fmt.Printf("✗ %v\n", err)
		return nil // up-by-one 은 nothing-to-do 도 에러로 잡지 않음
	}
	fmt.Printf("✓ applied 1 migration\n")
	return nil
}

func downHandler(ctx context.Context, db *sql.DB, t dbTarget) error {
	fmt.Printf("=== %s ===\n", t.label)
	if err := migrations.MigrateDown(ctx, db, t.cat); err != nil {
		return err
	}
	fmt.Printf("✓ rolled back 1 migration\n")
	return nil
}

func statusHandler(ctx context.Context, db *sql.DB, t dbTarget) error {
	fmt.Printf("=== %s ===\n", t.label)
	rows, err := migrations.Status(ctx, db, t.cat)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("(마이그레이션 파일 없음)")
		return nil
	}
	pending := 0
	for _, r := range rows {
		mark := "✓"
		if !r.Applied {
			mark = "·"
			pending++
		}
		// 루트 init 파일은 Version="" — 버전 prefix 없이 표시.
		display := r.Filename
		if r.Version != "" {
			display = r.Version + "/" + r.Filename
		}
		fmt.Printf("  %s  %s\n", mark, display)
	}
	fmt.Printf("→ total=%d, pending=%d\n", len(rows), pending)
	return nil
}

func baselineHandler(ctx context.Context, db *sql.DB, t dbTarget) error {
	fmt.Printf("=== %s ===\n", t.label)
	n, err := migrations.Baseline(ctx, db, t.cat)
	if err != nil {
		return err
	}
	fmt.Printf("✓ marked %d migration(s) as applied (no SQL executed)\n", n)
	return nil
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

func targetsFromFlag(s string) targetSet {
	switch s {
	case "all":
		return targetSet{game: true, shard: true}
	case "game":
		return targetSet{game: true}
	case "shard":
		return targetSet{shard: true}
	}
	return targetSet{}
}

// dbTargetsFor config 의 databases 를 game/shard 카테고리로 분류.
// shard_id == 1 (GameDBShardID) → game, 그 외 → shard.
func dbTargetsFor(cfg *config.Config, tg targetSet) ([]dbTarget, error) {
	gameID := int8(database.GameDBShardID)
	var out []dbTarget
	for _, c := range cfg.Databases {
		isGame := c.ShardID == gameID
		switch {
		case isGame && tg.game:
			out = append(out, dbTarget{
				label: fmt.Sprintf("GameDB[%s]", c.DBName),
				cat:   migrations.CategoryGame,
				cfg:   c,
			})
		case !isGame && tg.shard:
			out = append(out, dbTarget{
				label: fmt.Sprintf("ShardDB[%s]", c.DBName),
				cat:   migrations.CategoryShard,
				cfg:   c,
			})
		}
	}
	// 게임 DB 먼저, 그 다음 샤드 (shard_id 순).
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := out[i].cat == migrations.CategoryGame, out[j].cat == migrations.CategoryGame
		if ai != aj {
			return ai
		}
		return out[i].cfg.ShardID < out[j].cfg.ShardID
	})
	return out, nil
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", a...)
	os.Exit(1)
}

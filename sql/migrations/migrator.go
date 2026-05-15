package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/pressly/goose/v3"
)

// gooseMu goose 의 글로벌 상태(SetBaseFS/SetDialect)를 보호한다.
// 마이그레이터 CLI 는 단일 프로세스 내에서 game → shard 들을 순차 호출하므로
// 이 mutex 는 사실상 직렬화를 명시적으로 표현하는 용도.
var gooseMu sync.Mutex

// MigrationCategory 마이그레이션 디렉토리 카테고리 (게임 DB 용 / 샤드 DB 용).
type MigrationCategory string

const (
	CategoryGame  MigrationCategory = "game"
	CategoryShard MigrationCategory = "shard"
)

// MigrationFile 디렉토리 walker 가 반환하는 한 개 마이그레이션 파일 정보.
type MigrationFile struct {
	Version   string // 디렉토리 이름 — x.yy.zz
	Filename  string // 파일명 (timestamp_author_comment.sql)
	VersionID int64  // goose version_id (파일명 앞쪽 timestamp)
}

// MigrateUp 지정된 카테고리의 모든 마이그레이션을 순서대로 적용한다.
// 순서: 루트 init 파일들(timestamp 순) → 버전 디렉토리들(lexicographic 순) 내부의 파일들.
// goose.WithAllowMissing 을 사용해 버전 디렉토리 간 timestamp 역전을 허용.
func MigrateUp(ctx context.Context, db *sql.DB, cat MigrationCategory) (int, error) {
	gooseMu.Lock()
	defer gooseMu.Unlock()

	if err := goose.SetDialect("mysql"); err != nil {
		return 0, fmt.Errorf("set dialect: %w", err)
	}

	applied := 0
	// 1. 루트의 init 파일들 먼저 (goose 가 같은 디렉토리 안의 모든 *.sql 을 한 번에 처리).
	hasRoot, err := hasRootSQL(string(cat))
	if err != nil {
		return 0, err
	}
	if hasRoot {
		if err := upDir(ctx, db, string(cat)); err != nil {
			return applied, fmt.Errorf("%s/ (root): %w", cat, err)
		}
		applied++
	}

	// 2. 버전 디렉토리 순회.
	versions, err := listVersionDirs(string(cat))
	if err != nil {
		return applied, err
	}
	for _, v := range versions {
		if err := upDir(ctx, db, string(cat)+"/"+v); err != nil {
			return applied, fmt.Errorf("%s/%s: %w", cat, v, err)
		}
		applied++
	}
	return applied, nil
}

// upDir 한 디렉토리 안의 모든 *.sql 을 goose 로 적용. 호출자가 gooseMu 보유 + dialect 설정 가정.
func upDir(ctx context.Context, db *sql.DB, dirPath string) error {
	sub, err := fs.Sub(FS, dirPath)
	if err != nil {
		return fmt.Errorf("fs.Sub %s: %w", dirPath, err)
	}
	goose.SetBaseFS(sub)
	return goose.UpContext(ctx, db, ".", goose.WithAllowMissing())
}

// MigrateDown 가장 최근에 적용된 마이그레이션을 1 개 되돌린다.
// 루트 + 모든 버전 디렉토리를 후보로 검색해 current version_id 가 위치한 디렉토리에서 Down.
func MigrateDown(ctx context.Context, db *sql.DB, cat MigrationCategory) error {
	gooseMu.Lock()
	defer gooseMu.Unlock()

	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	current, err := currentVersion(ctx, db)
	if err != nil {
		return err
	}
	if current == 0 {
		return fmt.Errorf("nothing to rollback (current version = 0)")
	}

	// 검색 후보 디렉토리: 루트 → 버전 디렉토리들.
	candidates := []string{string(cat)}
	versions, err := listVersionDirs(string(cat))
	if err != nil {
		return err
	}
	for _, v := range versions {
		candidates = append(candidates, string(cat)+"/"+v)
	}

	for _, dir := range candidates {
		entries, err := fs.ReadDir(FS, dir)
		if err != nil {
			return err
		}
		for _, f := range entries {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
				continue
			}
			vid, ok := parseVersionID(f.Name())
			if !ok || vid != current {
				continue
			}
			sub, _ := fs.Sub(FS, dir)
			goose.SetBaseFS(sub)
			return goose.DownContext(ctx, db, ".")
		}
	}
	return fmt.Errorf("rollback target version_id %d not found in any migration dir", current)
}

// hasRootSQL 루트 디렉토리에 *.sql 이 하나라도 있는지.
func hasRootSQL(root string) (bool, error) {
	entries, err := fs.ReadDir(FS, root)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			return true, nil
		}
	}
	return false, nil
}

// MigrateUpByOne pending 마이그레이션 1 개만 적용.
// 검색 순서: 루트 → 버전 디렉토리들.
// 디버깅용 — 정상 워크플로는 MigrateUp.
func MigrateUpByOne(ctx context.Context, db *sql.DB, cat MigrationCategory) error {
	gooseMu.Lock()
	defer gooseMu.Unlock()

	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	candidates := []string{string(cat)}
	versions, err := listVersionDirs(string(cat))
	if err != nil {
		return err
	}
	for _, v := range versions {
		candidates = append(candidates, string(cat)+"/"+v)
	}

	for _, dir := range candidates {
		sub, _ := fs.Sub(FS, dir)
		goose.SetBaseFS(sub)
		err := goose.UpByOneContext(ctx, db, ".", goose.WithAllowMissing())
		if err == nil {
			return nil
		}
		// "no migrations to run" 은 다음 후보로.
		if strings.Contains(err.Error(), "no migrations") || err == goose.ErrNoNextVersion {
			continue
		}
		return fmt.Errorf("%s: %w", dir, err)
	}
	return fmt.Errorf("nothing to apply in %s/", cat)
}

// MigrationStatus 한 마이그레이션 파일의 적용 상태.
type MigrationStatus struct {
	Version   string // 디렉토리
	Filename  string
	VersionID int64
	Applied   bool
}

// Status 카테고리 내 모든 마이그레이션의 적용 여부를 반환 (디렉토리 + timestamp 순 정렬).
func Status(ctx context.Context, db *sql.DB, cat MigrationCategory) ([]MigrationStatus, error) {
	gooseMu.Lock()
	defer gooseMu.Unlock()

	if err := goose.SetDialect("mysql"); err != nil {
		return nil, fmt.Errorf("set dialect: %w", err)
	}

	// goose_db_version 테이블 존재 확인용 — 없으면 빈 set 반환 (모두 pending 으로 처리).
	applied, err := loadAppliedVersionIDs(ctx, db)
	if err != nil {
		return nil, err
	}

	files, err := WalkMigrationFiles(cat)
	if err != nil {
		return nil, err
	}

	out := make([]MigrationStatus, 0, len(files))
	for _, f := range files {
		out = append(out, MigrationStatus{
			Version:   f.Version,
			Filename:  f.Filename,
			VersionID: f.VersionID,
			Applied:   applied[f.VersionID],
		})
	}
	return out, nil
}

// CountPending Status 의 결과에서 pending 개수만.
func CountPending(ctx context.Context, db *sql.DB, cat MigrationCategory) (int, error) {
	st, err := Status(ctx, db, cat)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range st {
		if !s.Applied {
			n++
		}
	}
	return n, nil
}

// Baseline 카테고리 내 모든 마이그레이션을 *실행하지 않고* 적용 완료로 마킹한다.
// 이미 스키마가 갖춰진 기존 dev DB 를 goose 관리 체계로 편입할 때 1 회 호출.
func Baseline(ctx context.Context, db *sql.DB, cat MigrationCategory) (int, error) {
	gooseMu.Lock()
	defer gooseMu.Unlock()

	if err := goose.SetDialect("mysql"); err != nil {
		return 0, fmt.Errorf("set dialect: %w", err)
	}

	// goose_db_version 테이블이 없을 수도 있음 → 만들기 위해 NoVersioning 없이 Status 호출하면
	// 자동 생성됨. 그 다음 직접 INSERT.
	// 트릭: 빈 디렉토리에 대해 Up 호출하면 goose 가 schema 테이블만 만들고 끝.
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS goose_db_version (
		id BIGINT NOT NULL AUTO_INCREMENT,
		version_id BIGINT NOT NULL,
		is_applied BOOLEAN NOT NULL,
		tstamp TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY(id)
	) ENGINE=InnoDB`); err != nil {
		return 0, fmt.Errorf("create goose_db_version: %w", err)
	}
	// version_id=0 은 goose 가 표시하는 initial state — 없으면 추가.
	if _, err := db.ExecContext(ctx, `INSERT IGNORE INTO goose_db_version (id, version_id, is_applied) VALUES (1, 0, true)`); err != nil {
		return 0, fmt.Errorf("seed goose_db_version: %w", err)
	}

	files, err := WalkMigrationFiles(cat)
	if err != nil {
		return 0, err
	}
	applied, err := loadAppliedVersionIDs(ctx, db)
	if err != nil {
		return 0, err
	}
	marked := 0
	for _, f := range files {
		if applied[f.VersionID] {
			continue
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, true)`,
			f.VersionID,
		); err != nil {
			return marked, fmt.Errorf("baseline mark %d: %w", f.VersionID, err)
		}
		marked++
	}
	return marked, nil
}

// WalkMigrationFiles 카테고리 내 모든 마이그레이션 파일을 적용 순서로 나열.
// 순서: 루트 init (timestamp ASC) → 버전 디렉토리들(이름 ASC, 내부 timestamp ASC).
// 루트 파일의 MigrationFile.Version 은 빈 문자열.
func WalkMigrationFiles(cat MigrationCategory) ([]MigrationFile, error) {
	var out []MigrationFile

	// 1. 루트 init 파일들.
	rootFiles, err := collectSQLFiles(string(cat), "")
	if err != nil {
		return nil, err
	}
	out = append(out, rootFiles...)

	// 2. 버전 디렉토리들.
	versions, err := listVersionDirs(string(cat))
	if err != nil {
		return nil, err
	}
	for _, v := range versions {
		vFiles, err := collectSQLFiles(string(cat)+"/"+v, v)
		if err != nil {
			return nil, err
		}
		out = append(out, vFiles...)
	}
	return out, nil
}

// collectSQLFiles 디렉토리 내의 *.sql 파일들을 timestamp 오름차순으로 반환.
// dirPath: embed.FS 내 경로. label: MigrationFile.Version 에 들어갈 값 (루트는 빈 문자열).
func collectSQLFiles(dirPath, label string) ([]MigrationFile, error) {
	entries, err := fs.ReadDir(FS, dirPath)
	if err != nil {
		return nil, err
	}
	var files []MigrationFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		vid, ok := parseVersionID(e.Name())
		if !ok {
			return nil, fmt.Errorf("invalid migration filename: %s/%s (expected <timestamp>_...)", dirPath, e.Name())
		}
		files = append(files, MigrationFile{
			Version:   label,
			Filename:  e.Name(),
			VersionID: vid,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].VersionID < files[j].VersionID
	})
	return files, nil
}

// listVersionDirs 카테고리 root 아래의 버전 디렉토리들을 lexicographic 정렬해 반환.
func listVersionDirs(root string) ([]string, error) {
	entries, err := fs.ReadDir(FS, root)
	if err != nil {
		return nil, fmt.Errorf("read migrations/%s: %w", root, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// parseVersionID 파일명 앞 14 자리 timestamp 를 int64 로 파싱. ex) "20260520143012_mega_xxx.sql"
func parseVersionID(filename string) (int64, bool) {
	idx := strings.Index(filename, "_")
	if idx <= 0 {
		return 0, false
	}
	tsStr := filename[:idx]
	if len(tsStr) != 14 {
		return 0, false
	}
	var v int64
	for _, ch := range tsStr {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		v = v*10 + int64(ch-'0')
	}
	return v, true
}

// currentVersion goose_db_version 테이블의 MAX(version_id). 테이블 없으면 0.
func currentVersion(ctx context.Context, db *sql.DB) (int64, error) {
	var v sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = true`,
	).Scan(&v)
	if err != nil {
		// 테이블 없으면 0 으로 취급.
		if isTableNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return v.Int64, nil
}

// loadAppliedVersionIDs goose_db_version 의 is_applied=true 행들을 set 으로.
func loadAppliedVersionIDs(ctx context.Context, db *sql.DB) (map[int64]bool, error) {
	out := map[int64]bool{}
	rows, err := db.QueryContext(ctx,
		`SELECT version_id FROM goose_db_version WHERE is_applied = true`)
	if err != nil {
		if isTableNotFound(err) {
			return out, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func isTableNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "1146") // MySQL ER_NO_SUCH_TABLE
}

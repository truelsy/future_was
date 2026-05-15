package migrations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CLI (cmd/migrator) 와 admin 엔드포인트 (router/admin.go) 가 공유하는
// "새 마이그레이션 SQL 파일 생성" 로직.
//
// 디스크에 파일을 쓰고 경로를 반환. 실행 중인 서버의 embed.FS 는 빌드 시점에 박힌
// 상태라 새 파일이 즉시 반영되지 않으므로, 호출자는 안내 메시지를 별도로 전달해야 한다
// (다음 `make mig-up` 시 go run 이 재빌드하며 embed 됨).

// Sentinel errors — HTTP 핸들러가 400/409 등으로 분기 매핑할 때 사용.
var (
	ErrInvalidCategory = errors.New("category must be 'game' or 'shard'")
	ErrInvalidVersion  = errors.New("invalid version format (expected x.yy.zz)")
	ErrInvalidName     = errors.New("name must be snake_case")
	ErrAuthorRequired  = errors.New("author required (or invalid after sanitize)")
	ErrFileExists      = errors.New("migration file already exists")
)

// 외부에서 재사용 가능한 패턴.
var (
	VersionPattern = regexp.MustCompile(`^\d+\.\d{2}\.\d{2}$`)
	NamePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)
)

// kstZone 마이그레이션 timestamp 용 고정 타임존 (UTC+9, DST 없음).
// time.LoadLocation 대신 FixedZone 사용 — OS 의 tzdata 의존 제거.
var kstZone = time.FixedZone("KST", 9*60*60)

// CreateRequest 새 마이그레이션 파일 생성 요청 인자.
type CreateRequest struct {
	Category MigrationCategory // CategoryGame | CategoryShard
	Version  string            // x.yy.zz
	Name     string            // snake_case (verb_object 권장: add_*, drop_*, …)
	Author   string            // 영문 소문자/숫자/언더스코어. sanitize 후 비면 ErrAuthorRequired
	BaseDir  string            // 호출자가 결정 (보통 "sql/migrations"). 프로젝트 루트 기준 상대 경로.

	// SQL 본문 (옵션). 비어있으면 헤더 + 빈 +goose Up/Down 마커만 들어간 템플릿이 생성됨.
	// 호출자는 "-- +goose Up" 마커를 직접 넣지 말 것 — 헬퍼가 wrap.
	UpSQL   string
	DownSQL string
}

// CreateMigrationFile 새 마이그레이션 SQL 파일을 디스크에 생성한다.
// 반환: 생성된 파일의 상대 경로 (BaseDir 포함).
func CreateMigrationFile(req CreateRequest) (string, error) {
	if req.Category != CategoryGame && req.Category != CategoryShard {
		return "", ErrInvalidCategory
	}
	if !VersionPattern.MatchString(req.Version) {
		return "", ErrInvalidVersion
	}
	if !NamePattern.MatchString(req.Name) {
		return "", ErrInvalidName
	}
	author := SanitizeAuthor(req.Author)
	if author == "" {
		return "", ErrAuthorRequired
	}

	// 파일명 timestamp 와 헤더의 created 모두 KST 기준.
	now := time.Now().In(kstZone)
	ts := now.Format("20060102150405")
	filename := fmt.Sprintf("%s_%s_%s.sql", ts, author, req.Name)
	dir := filepath.Join(req.BaseDir, string(req.Category), req.Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err == nil {
		return "", ErrFileExists
	}

	content := buildTemplate(req.Name, author, now, req.UpSQL, req.DownSQL)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	return path, nil
}

// buildTemplate goose 어노테이션을 포함한 SQL 본문을 조립.
// upSQL / downSQL 이 빈 문자열이면 마커 아래 빈 줄만 들어가 IDE 에서 채우면 되는 상태.
// createdAt 은 호출자가 결정한 KST 시각 (파일명 timestamp 와 일치하도록).
func buildTemplate(name, author string, createdAt time.Time, upSQL, downSQL string) string {
	upBody := strings.TrimRight(upSQL, " \n\t")
	downBody := strings.TrimRight(downSQL, " \n\t")

	upBlock := ""
	if upBody != "" {
		upBlock = upBody + "\n"
	}
	downBlock := ""
	if downBody != "" {
		downBlock = downBody + "\n"
	}

	return fmt.Sprintf(`-- migration: %s
-- author:    %s
-- created:   %s

-- +goose Up
%s

-- +goose Down
%s
`, name, author, createdAt.Format("2006-01-02 15:04:05 KST"), upBlock, downBlock)
}

// SanitizeAuthor 영문 소문자/숫자/언더스코어만 남기고 그 외는 제거.
// 'A-Z' 는 소문자화, hyphen 은 underscore 로 치환.
func SanitizeAuthor(s string) string {
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

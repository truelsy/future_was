// Package migrations 는 모든 마이그레이션 SQL 파일을 Go 바이너리에 embed 한다.
//
// 디렉토리 구조:
//
//	<db>/<시간>_init_<...>.sql                       ← baseline (루트, 버전 디렉토리 밖)
//	<db>/<버전>/<시간>_<작업자>_<코멘트>.sql        ← 릴리즈별 변경
//
// 적용 순서:
//  1. 루트 파일을 timestamp 오름차순으로 먼저 적용 (init).
//  2. 버전 디렉토리들을 lexicographic 정렬해 순차 적용.
//
// 컨벤션:
//   - db: "game" | "shard"
//   - 버전: x.yy.zz (예: 2.01.02) — TB_VERSION 의 client_version 포맷
//   - 시간: YYYYMMDDhhmmss — goose 의 version_id
//   - 작업자: git config user.email 의 로컬 파트 (예: mega)
//   - 코멘트: snake_case 동작 설명 (예: add_account_last_seen)
//
// 새 카테고리(예: analytics) 추가 시 //go:embed 패턴에 두 줄(루트 + 서브디렉토리) 추가.
package migrations

import (
	"embed"
	"io/fs"
)

// game/ 와 shard/ 디렉토리 트리 전체를 재귀 embed.
// 새 카테고리(예: analytics) 추가 시 디렉토리명 한 단어만 추가하면 됨.
//
//go:embed game shard
var defaultFS embed.FS

// FS 마이그레이션 파일 소스. 기본은 빌드 시 embed 된 정적 FS.
// 로컬 dev 환경에서 SetSource(os.DirFS("sql/migrations")) 로 교체 시
// 디스크의 최신 파일을 재빌드 없이 즉시 반영 가능 (admin UI status / 파일 뷰어 등).
// 다른 환경에서는 embed 그대로 — 배포된 바이너리와 동일한 파일셋 보장.
var FS fs.FS = defaultFS

// SetSource 마이그레이션 소스 FS 를 교체한다.
// 보통 main.go 에서 stage=local 일 때 1 회 호출. 동시성 안전성 X — 시작 시점에만 사용.
func SetSource(src fs.FS) {
	FS = src
}

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

import "embed"

// game/ 와 shard/ 디렉토리 트리 전체를 재귀 embed.
// 새 카테고리(예: analytics) 추가 시 디렉토리명 한 단어만 추가하면 됨.
//
//go:embed game shard
var FS embed.FS

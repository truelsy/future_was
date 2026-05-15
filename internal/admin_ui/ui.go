// Package admin_ui 어드민 React SPA 산출물을 Go 바이너리에 embed 한다.
//
// 빌드 흐름:
//
//	cd web/admin && npm run build        // outDir 이 ../../internal/admin_ui/dist
//	go build ./...                       // dist 의 정적 파일이 바이너리에 포함
//
// 런타임: router/admin.go 가 /admin/ui/* 로 정적 서빙.
package admin_ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS dist 하위만 잘라 반환한다 — http.FileServer 가 dist 를 root 로 보게.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// dist 디렉토리 자체는 placeholder(index.html) 가 항상 존재하므로 도달 불가.
		panic(err)
	}
	return sub
}

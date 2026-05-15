package router

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"future_was/config"
	"future_was/internal/admin_ui"
	"future_was/internal/clock"
	"future_was/internal/container"
	"future_was/internal/database"
	"future_was/internal/log"
	"future_was/internal/model"
	"future_was/internal/repository"
	"future_was/sql/migrations"

	"github.com/labstack/echo/v4"
)

// setupAdmin 관리자 전용 엔드포인트를 등록한다.
// live 환경에서는 admin 기능을 *전혀 노출하지 않는다* — /admin 경로 전체가 404.
// 공격 표면을 최소화하고 의도적으로만 운영 도구를 노출.
// TODO: 인증 미들웨어(adminAuth)를 추가해야 한다.
func setupAdmin(e *echo.Echo, c *container.Container) {
	if c.Stage.IsLive() {
		log.Info().Msg("admin endpoints disabled (stage=live)")
		return
	}
	log.Info().Msgf("admin endpoints enabled (stage=%s)", c.Stage)

	g := e.Group("/admin")

	// POST /admin/design/reload
	// TB_VERSION의 is_active=1 행을 다시 조회하여 디자인을 재로드하고,
	// Redis Pub/Sub로 다른 서버에도 reload 신호를 보낸다.
	g.POST("/design/reload", func(ec echo.Context) error {
		if err := c.DesignSyncer.Trigger(ec.Request().Context()); err != nil {
			return ec.String(http.StatusInternalServerError, err.Error())
		}
		return ec.String(http.StatusOK, "OK: reloaded from TB_VERSION")
	})

	g.POST("/resource/reload", func(ec echo.Context) error {
		if err := c.ResourceSyncer.Trigger(ec.Request().Context()); err != nil {
			return ec.String(http.StatusInternalServerError, err.Error())
		}
		return ec.String(http.StatusOK, "OK: resource reloaded")
	})

	// GET /admin/info  서버 메타 정보 (stage 등). 프론트가 환경별 분기에 사용.
	g.GET("/info", func(ec echo.Context) error {
		return ec.JSON(http.StatusOK, map[string]string{
			"stage": string(c.Stage),
		})
	})

	registerClockAdmin(g, c)
	registerVersionAdmin(g, c)
	registerMigrationsAdmin(g, c)
	registerAdminUI(g)
}

// registerMigrationsAdmin DB 마이그레이션 상태 조회 (read-only) + 파일 생성 (로컬 전용).
// 실제 적용/롤백은 `make mig-up` CLI 만 지원. 운영 안전상 admin 페이지에서는 노출 X.
func registerMigrationsAdmin(g *echo.Group, c *container.Container) {
	type fileStatus struct {
		Version   string `json:"version"` // 빈 문자열 = 루트 init
		Filename  string `json:"filename"`
		VersionID int64  `json:"version_id"`
		Applied   bool   `json:"applied"`
	}
	type dbStatus struct {
		Label      string       `json:"label"`    // 표시명 — "GameDB[N_GAME]" 등
		Category   string       `json:"category"` // "game" | "shard"
		ShardID    int8         `json:"shard_id"`
		DBName     string       `json:"db_name"`
		Total      int          `json:"total"`
		Pending    int          `json:"pending"`
		Migrations []fileStatus `json:"migrations"`
		Error      string       `json:"error,omitempty"` // 한 DB 만 실패해도 다른 DB 결과는 반환
	}

	// path traversal 차단용 — 마이그레이션 파일 경로 검증.
	versionRE := regexp.MustCompile(`^\d+\.\d{2}\.\d{2}$`)
	filenameRE := regexp.MustCompile(`^\d{14}_[a-z0-9_]+\.sql$`)

	// GET /admin/migrations/file?category=game|shard&version=2.01.02&filename=YYYYMMDDhhmmss_author_comment.sql
	// 파일 내용을 그대로 반환. version 이 빈 문자열이면 루트(init) 파일.
	g.GET("/migrations/file", func(ec echo.Context) error {
		cat := ec.QueryParam("category")
		ver := ec.QueryParam("version")
		filename := ec.QueryParam("filename")

		if cat != "game" && cat != "shard" {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": "category must be 'game' or 'shard'"})
		}
		if ver != "" && !versionRE.MatchString(ver) {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": "invalid version format (expected x.yy.zz)"})
		}
		if !filenameRE.MatchString(filename) {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": "invalid filename format"})
		}

		// 경로 조립: <cat>/[<ver>/]<filename>
		path := cat + "/"
		if ver != "" {
			path += ver + "/"
		}
		path += filename

		data, err := migrations.FS.ReadFile(path)
		if err != nil {
			return ec.JSON(http.StatusNotFound, map[string]string{"error": "file not found: " + path})
		}
		return ec.JSON(http.StatusOK, map[string]any{
			"path":    path,
			"content": string(data),
		})
	})

	// POST /admin/migrations  새 마이그레이션 SQL 파일을 디스크에 생성.
	// **로컬 전용** — 다른 환경에서는 git tree 가 없어 파일 생성이 무의미하므로 차단.
	// 서버는 프로젝트 루트에서 실행 중이어야 sql/migrations/ 경로가 올바르게 해석됨.
	// 생성된 파일은 다음 `make mig-up` 시 go run 이 재빌드하면서 embed.FS 에 반영됨.
	g.POST("/migrations", func(ec echo.Context) error {
		if c.Stage != config.StageLocal {
			return ec.JSON(http.StatusForbidden, map[string]string{
				"error": fmt.Sprintf("migration file creation is only allowed in local stage (current: %s)", c.Stage),
			})
		}
		var req struct {
			Category string `json:"category"` // "game" | "shard"
			Version  string `json:"version"`  // x.yy.zz
			Name     string `json:"name"`     // snake_case
			Author   string `json:"author"`
			UpSQL    string `json:"up_sql"`
			DownSQL  string `json:"down_sql"`
		}
		if err := ec.Bind(&req); err != nil {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		path, err := migrations.CreateMigrationFile(migrations.CreateRequest{
			Category: migrations.MigrationCategory(req.Category),
			Version:  req.Version,
			Name:     req.Name,
			Author:   req.Author,
			BaseDir:  "sql/migrations",
			UpSQL:    req.UpSQL,
			DownSQL:  req.DownSQL,
		})
		if err != nil {
			switch {
			case errors.Is(err, migrations.ErrFileExists):
				return ec.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
			case errors.Is(err, migrations.ErrInvalidCategory),
				errors.Is(err, migrations.ErrInvalidVersion),
				errors.Is(err, migrations.ErrInvalidName),
				errors.Is(err, migrations.ErrAuthorRequired):
				return ec.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			default:
				return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
		return ec.JSON(http.StatusOK, map[string]string{"path": path})
	})

	// GET /admin/migrations/status  game + 모든 shard 의 적용 상태.
	g.GET("/migrations/status", func(ec echo.Context) error {
		ctx := ec.Request().Context()
		var out []dbStatus

		for _, s := range database.AllShards() {
			cat := migrations.CategoryShard
			label := "ShardDB[" + s.DBName + "]"
			if s.ShardID == database.GameDBShardID {
				cat = migrations.CategoryGame
				label = "GameDB[" + s.DBName + "]"
			}

			ds := dbStatus{
				Label:    label,
				Category: string(cat),
				ShardID:  s.ShardID,
				DBName:   s.DBName,
			}

			rows, err := migrations.Status(ctx, s.DB.SqlxDB().DB, cat)
			if err != nil {
				ds.Error = err.Error()
				out = append(out, ds)
				continue
			}
			ds.Total = len(rows)
			for _, r := range rows {
				ds.Migrations = append(ds.Migrations, fileStatus{
					Version:   r.Version,
					Filename:  r.Filename,
					VersionID: r.VersionID,
					Applied:   r.Applied,
				})
				if !r.Applied {
					ds.Pending++
				}
			}
			out = append(out, ds)
		}
		return ec.JSON(http.StatusOK, out)
	})
}

// registerVersionAdmin TB_VERSION 행 추가/조회.
// 행 추가만 한다 — 실제 catalog 로드는 별도로 POST /admin/design/reload 호출.
func registerVersionAdmin(g *echo.Group, c *container.Container) {
	repo := repository.NewVersionRepository(c.GameDB)

	// GET /admin/versions  최근 200건 (idx DESC).
	g.GET("/versions", func(ec echo.Context) error {
		rows, err := repo.FindAll()
		if err != nil {
			return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return ec.JSON(http.StatusOK, rows)
	})

	// DELETE /admin/version/:idx  지정 idx 행을 삭제.
	// 삭제 후 Catalog 반영은 별도로 POST /admin/design/reload 호출.
	g.DELETE("/version/:idx", func(ec echo.Context) error {
		idxStr := ec.Param("idx")
		idx, err := strconv.ParseUint(idxStr, 10, 64)
		if err != nil {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": "invalid idx"})
		}
		affected, err := repo.Delete(idx)
		if err != nil {
			return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if affected == 0 {
			return ec.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
		}
		return ec.JSON(http.StatusOK, map[string]any{"idx": idx, "deleted": affected})
	})

	// POST /admin/version
	// body: { "client_version", "server_version", "app_id", "is_active",
	//        "update_flag", "inspection_flag", "catalog_filename", "comment" }
	g.POST("/version", func(ec echo.Context) error {
		var req struct {
			ClientVersion   string `json:"client_version"`
			ServerVersion   string `json:"server_version"`
			AppID           string `json:"app_id"`
			IsActive        uint8  `json:"is_active"`
			UpdateFlag      uint8  `json:"update_flag"`
			InspectionFlag  uint8  `json:"inspection_flag"`
			CatalogFilename string `json:"catalog_filename"`
			Comment         string `json:"comment"`
		}
		if err := ec.Bind(&req); err != nil {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if req.ClientVersion == "" || req.ServerVersion == "" {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": "client_version, server_version required"})
		}

		m := &model.Version{
			ClientVersion:   req.ClientVersion,
			ServerVersion:   req.ServerVersion,
			AppID:           req.AppID,
			IsActive:        req.IsActive,
			UpdateFlag:      req.UpdateFlag,
			InspectionFlag:  req.InspectionFlag,
			CatalogFilename: req.CatalogFilename,
			Comment:         req.Comment,
		}
		if err := repo.Create(m); err != nil {
			return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return ec.JSON(http.StatusOK, m)
	})
}

// registerAdminUI React SPA 정적 파일 서빙.
// 빌드 산출물(internal/admin_ui/dist) 이 Go 바이너리에 embed 되어 있다.
// SPA 라우팅을 위해 알 수 없는 경로는 index.html 로 fallback 한다 (BrowserRouter 호환).
//
// /admin/ui/clock 같이 브라우저 주소창에 직접 입력해도 index.html 로딩 →
// React Router 가 클라이언트에서 라우팅한다.
func registerAdminUI(g *echo.Group) {
	uiFS := admin_ui.FS()

	g.GET("/ui", func(ec echo.Context) error {
		return ec.Redirect(http.StatusFound, "/admin/ui/")
	})

	g.GET("/ui/*", func(ec echo.Context) error {
		p := strings.TrimPrefix(ec.Request().URL.Path, "/admin/ui/")
		if p == "" {
			p = "index.html"
		}
		// 파일이 존재하지 않거나 디렉토리면 SPA fallback 으로 index.html 반환.
		info, err := fs.Stat(uiFS, p)
		if errors.Is(err, fs.ErrNotExist) || (err == nil && info.IsDir()) {
			p = "index.html"
		}
		http.ServeFileFS(ec.Response(), ec.Request(), uiFS, p)
		return nil
	})
}

// registerClockAdmin 서버 시간 오프셋 admin API.
// 호출받은 서버에만 즉시 적용. 다른 서버 인스턴스는 재시작 시 DB에서 로드.
func registerClockAdmin(g *echo.Group, c *container.Container) {
	repo := repository.NewAddServerTimeRepository(c.GameDB)

	// POST /admin/clock/jump
	// body: { "add_second": 86400, "user": "qa1" }
	// add_second 는 *절대 오프셋(초)*. 그대로 clock 오프셋에 적용된다 (덮어쓰기).
	// 0 을 보내면 원복.
	g.POST("/clock/jump", func(ec echo.Context) error {
		var req struct {
			AddSecond int64  `json:"add_second"`
			User      string `json:"user"`
		}
		if err := ec.Bind(&req); err != nil {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if req.User == "" {
			return ec.JSON(http.StatusBadRequest, map[string]string{"error": "user required"})
		}

		if err := repo.Create(req.AddSecond, req.User); err != nil {
			return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		clock.SetOffset(time.Duration(req.AddSecond) * time.Second)

		return ec.JSON(http.StatusOK, map[string]any{
			"add_second":   req.AddSecond,
			"logical_time": clock.Now().Format(time.RFC3339),
		})
	})

	// GET /admin/clock  현재 오프셋/시간 조회
	g.GET("/clock", func(ec echo.Context) error {
		return ec.JSON(http.StatusOK, map[string]any{
			"offset_sec":   int64(clock.Offset().Seconds()),
			"offset_human": clock.Offset().String(),
			"real_time":    time.Now().Format(time.RFC3339),
			"logical_time": clock.Now().Format(time.RFC3339),
		})
	})
}

package router

import (
	"net/http"
	"strconv"
	"time"

	"future_was/internal/admin_ui"
	"future_was/internal/clock"
	"future_was/internal/container"
	"future_was/internal/model"
	"future_was/internal/repository"

	"github.com/labstack/echo/v4"
)

// setupAdmin 관리자 전용 엔드포인트를 등록한다.
// TODO: 인증 미들웨어(adminAuth)를 추가해야 한다.
func setupAdmin(e *echo.Echo, c *container.Container) {
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

	registerClockAdmin(g, c)
	registerVersionAdmin(g, c)
	registerAdminUI(g)
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
func registerAdminUI(g *echo.Group) {
	uiFS := admin_ui.FS()
	fileServer := http.FileServer(http.FS(uiFS))
	stripped := http.StripPrefix("/admin/ui/", fileServer)

	g.GET("/ui", func(ec echo.Context) error {
		return ec.Redirect(http.StatusFound, "/admin/ui/")
	})
	g.GET("/ui/*", echo.WrapHandler(stripped))
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

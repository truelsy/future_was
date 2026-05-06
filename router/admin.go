package router

import (
	"net/http"

	"future_next_baseball/internal/container"

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
}

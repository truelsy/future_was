package middleware

import (
	"future_cpbl_web_server/internal/errcode"
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/internal/resource"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

func MaintenanceMiddleware(store *resource.Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path

			// /admin/* 은 점검 중에도 통과 (reload용)
			if strings.HasPrefix(path, "/admin/") {
				return next(c)
			}

			// 헬스체크/welcome 등은 점검 중에도 통과
			// (LB가 인스턴스를 unhealthy로 마킹하지 않도록)
			switch path {
			case "/", "/health", "/favicon.ico":
				return next(c)
			}

			now := uint32(time.Now().Unix())
			if m := store.Get().ActiveMaintenance(now); m != nil {
				// 점검중
				return handler.SendGameError(c, 0, errcode.CodeMaintenance, m.Msg)
			}

			return next(c)
		}
	}
}

package middleware

import (
	"fmt"
	"future_next_baseball/internal/log"
	"net/http/httputil"
	"runtime/debug"
	"strings"

	"github.com/labstack/echo/v4"
)

func RecoverMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if err := recover(); err != nil {
					stack := strings.Split(string(debug.Stack()), "\n")
					req, _ := httputil.DumpRequest(c.Request(), false)

					log.Panic().Msgf("%s:%s\n\n%s", string(req), err, strings.Join(stack[4:], "\n"))
					if e, ok := err.(error); ok {
						c.Error(e)
					} else {
						c.Error(fmt.Errorf("%v", err))
					}
				}
			}()
			return next(c)
		}
	}
}

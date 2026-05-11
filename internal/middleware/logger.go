package middleware

import (
	"bytes"
	"future_next_baseball/internal/handler"
	"future_next_baseball/internal/log"
	"io"
	"time"

	"github.com/labstack/echo/v4"
)

func LogMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			// 로그 제외 경로
			switch c.Request().RequestURI {
			case "/", "/favicon.ico", "/health":
				return next(c)
			}

			startTime := time.Now()

			// 요청 body 복제 (디스패처에서 읽을 수 있도록 원본 복원)
			requestBody := new(bytes.Buffer)
			backBuffer := new(bytes.Buffer)
			cloneReader := io.TeeReader(c.Request().Body, backBuffer)
			if body, err := io.ReadAll(cloneReader); err == nil {
				requestBody.Write(body)
			}
			if requestBody.Len() == 0 {
				requestBody.Write([]byte("{}"))
			}
			c.Request().Body = io.NopCloser(backBuffer)

			// 핸들러 실행
			if err = next(c); err != nil {
				c.Error(err)
			}

			latency := time.Since(startTime).Seconds() * 1000 // ms

			event := log.Info().
				Uint32("action_id", handler.ActionID(c)).
				Uint64("user_id", handler.UserID(c)).
				Int("status", c.Response().Status).
				Float64("latency_ms", latency).
				Str("ip", c.RealIP()).
				Int("req_size", requestBody.Len()).
				Int64("res_size", c.Response().Size)

			if reqJSON := handler.ReqJSON(c); reqJSON != nil {
				event = event.RawJSON("req", reqJSON)
			}
			if resJSON := handler.ResJSON(c); resJSON != nil {
				event = event.RawJSON("res", resJSON)
			}

			event.Msg("api")

			return nil
		}
	}
}

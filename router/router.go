package router

import (
	"future_next_baseball/internal/container"
	"future_next_baseball/internal/handler"

	// 서브패키지의 init() 함수를 실행하여 액션을 등록하기 위한 blank import.
	_ "future_next_baseball/internal/handler/account"
	_ "future_next_baseball/internal/handler/card"

	"github.com/labstack/echo/v4"
)

func Setup(e *echo.Echo) {
	c := container.New()
	handler.SetupAll(e, c)
}

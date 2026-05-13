package router

import (
	"future_was/internal/container"
	"future_was/internal/handler"

	// 서브패키지의 init() 함수를 실행하여 액션을 등록하기 위한 blank import.
	_ "future_was/internal/handler/account"
	_ "future_was/internal/handler/card"
	_ "future_was/internal/handler/item"

	"github.com/labstack/echo/v4"
)

// Setup 핸들러를 초기화하고 라우트를 등록한다.
// container는 main에서 design 의존성과 함께 구성된 인스턴스를 전달한다.
func Setup(e *echo.Echo, c *container.Container) {
	handler.SetupAll(e, c)
	setupAdmin(e, c)
}

package handler

import (
	"future_next_baseball/internal/design"

	"github.com/labstack/echo/v4"
)

// Design context의 client_version으로 매핑된 디자인 Snapshot을 반환한다.
// 지원하지 않는 버전이면 nil. 호출부에서 nil 체크 + 적절한 에러 응답 처리.
func Design(c echo.Context) *design.Snapshot {
	version, _ := c.Get("client_version").(string)
	return ctn.DesignStore.GetByClientVersion(version)
}

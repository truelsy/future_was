package database

import (
	"database/sql"
	"errors"
)

// ErrNoRows sql.ErrNoRows를 편의상 재노출한 것이다.
var ErrNoRows = sql.ErrNoRows

// IsNotFound 에러가 "no rows" 에러인지 판별한다.
// 서비스/핸들러 레이어에서 database 패키지 직접 import를 피하기 위해 사용한다.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

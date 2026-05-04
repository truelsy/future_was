package util

import "time"

// ToDatetimeKST
// timestamp 를 datetime string 으로 변환한다.
func ToDatetimeKST(t int64) string {
	loc, _ := time.LoadLocation("Asia/Seoul")
	return time.Unix(t, 0).In(loc).Format("2006-01-02 15:04:05")
}

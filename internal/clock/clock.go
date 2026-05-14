// Package clock 비즈니스 로직용 "논리적 현재 시각"을 제공한다.
//
// 운영에서는 time.Now() 와 동일하게 동작하며, QA 환경에서 admin API 로
// 오프셋을 주입하면 비즈니스 시간(상품 판매·이벤트·출석 등)을 점프시킬 수 있다.
//
// 사용 구분:
//   - 비즈니스 시간(model 의 InsertTime/UpdateTime, 상품 expire 비교 등): clock.Now()
//   - 인프라 시간(lock/session TTL, 로그 timestamp, envelope timestamp): time.Now() 그대로
//
// 인프라까지 clock 으로 갈아끼우면 점프 시 lock TTL 만료/세션 무한 만료 같은 사고가 난다.
package clock

import (
	"sync/atomic"
	"time"
)

// offsetNs 추가 시간(ns). 음수면 과거로 점프. QA admin API 로만 변경.
var offsetNs atomic.Int64

// Now 오프셋이 적용된 현재 시각. 운영(offset=0)에서는 time.Now() 와 동일.
func Now() time.Time {
	return time.Now().Add(time.Duration(offsetNs.Load()))
}

// Offset 현재 적용된 오프셋.
func Offset() time.Duration {
	return time.Duration(offsetNs.Load())
}

// SetOffset 오프셋을 절대값으로 설정한다 (QA 전용).
func SetOffset(d time.Duration) {
	offsetNs.Store(int64(d))
}

// Jump 현재 오프셋에 d 만큼을 더한다 (QA 전용).
func Jump(d time.Duration) {
	offsetNs.Add(int64(d))
}

// Reset 오프셋을 0으로 되돌린다.
func Reset() {
	offsetNs.Store(0)
}

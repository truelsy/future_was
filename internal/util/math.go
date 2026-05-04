package util

import "math"

func Round(val float64, roundOn float64, places int) float64 {

	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)

	var round float64
	if val > 0 {
		if div >= roundOn {
			round = math.Ceil(digit)
		} else {
			round = math.Floor(digit)
		}
	} else {
		if div >= roundOn {
			round = math.Floor(digit)
		} else {
			round = math.Ceil(digit)
		}
	}

	return round / pow
}

func RoundDown(v float64) int32 {
	ret := math.Floor(v)
	return int32(ret)
}

func Min[T int | int16 | int32 | int64 | uint | uint16 | uint32 | uint64 | float32 | float64](a, b T) T {
	if a > b {
		return b
	}
	return a
}

func Max[T int | int16 | int32 | int64 | uint | uint16 | uint32 | uint64 | float32 | float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}

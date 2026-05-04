package util

import (
	"strconv"
)

// ToStr
// converts the argument to a string.
// int 종류와, bool 만 지원
func ToStr(arg interface{}) string {
	var str string
	switch val := arg.(type) {
	case int8:
		str = strconv.Itoa(int(val))
	case int16:
		str = strconv.Itoa(int(val))
	case int32:
		str = strconv.Itoa(int(val))
	case int:
		str = strconv.Itoa(val)
	case int64:
		str = strconv.FormatInt(val, 10)
	case uint8:
		str = strconv.Itoa(int(val))
	case uint16:
		str = strconv.Itoa(int(val))
	case uint32:
		str = strconv.Itoa(int(val))
	case uint:
		str = strconv.Itoa(int(val))
	case uint64:
		str = strconv.FormatUint(val, 10)
	case string:
		str = val
	case bool:
		if val {
			str = "1"
		} else {
			str = "0"
		}
	}
	return str
}

func ToInt64(val string) int64 {
	num, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		panic(err)
	}
	return num
}

func ToUint64(val string) uint64 {
	num, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		panic(err)
	}
	return num
}

func ToUint32(val string) uint32 {
	num, err := strconv.ParseUint(val, 10, 32)
	if err != nil {
		panic(err)
	}
	return uint32(num)
}

func ToInt32(val string) int32 {
	num, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		panic(err)
	}
	return int32(num)
}

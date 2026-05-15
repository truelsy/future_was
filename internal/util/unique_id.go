package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

func Uniqid(prefix string, moreEntropy bool) string {
	now := time.Now()
	sec := now.Unix()               // 초
	usec := now.Nanosecond() / 1000 // 마이크로초

	// prefix + 8자리(초) + 5자리(마이크로초) hex
	uid := fmt.Sprintf("%s%08x%05x", prefix, sec, usec)

	if moreEntropy {
		// crypto/rand로 0 ~ 99999999 사이 정수를 만들어 8자리로 포맷.
		n, err := rand.Int(rand.Reader, big.NewInt(100000000))
		if err != nil {
			// 실패하면 그냥 time 기반만 쓰도록 fallback
			return uid
		}
		uid = fmt.Sprintf("%s.%08d", uid, n.Int64())
	}

	return uid
}

// NewInstanceID 프로세스마다 고유한 16 자 hex ID 생성. self-publish 식별용.
func NewInstanceID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

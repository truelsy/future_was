package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// PotentialMap 은 PHP가 JSON으로 인코딩한 potential 데이터를 받기 위한 타입이다.
// PHP는 sequential int key array를 `[1,1]` 배열로, associative array를 `{"2":1,...}` 객체로
// 출력하므로 두 형태를 모두 흡수해 항상 map[id]value 형태로 정규화한다.
type PotentialMap map[uint32]uint32

// UnmarshalJSON 은 PHP의 potential 데이터를 PotentialMap으로 변환한다.
func (m *PotentialMap) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*m = PotentialMap{}
		return nil
	}

	switch data[0] {
	case '{':
		raw := map[string]uint32{}
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		out := make(PotentialMap, len(raw))
		for k, v := range raw {
			id, err := strconv.ParseUint(k, 10, 32)
			if err != nil {
				return fmt.Errorf("PotentialMap key %q: %w", k, err)
			}
			out[uint32(id)] = v
		}
		*m = out
		return nil
	case '[':
		var arr []uint32
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		out := make(PotentialMap, len(arr))
		for i, v := range arr {
			out[uint32(i)] = v
		}
		*m = out
		return nil
	default:
		return fmt.Errorf("PotentialMap: unexpected token %q", data[0])
	}
}

// MarshalJSON 은 항상 객체 형태로 직렬화하여 PHP/Go 간 일관성을 유지한다.
func (m PotentialMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[uint32]uint32(m))
}

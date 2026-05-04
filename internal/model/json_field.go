package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONField 는 MySQL JSON 컬럼을 위한 제네릭 래퍼이다.
// sql.Scanner driver.Valuer 를 구현하여 어떤 Go 타입이든
// JSON 컬럼으로 투명하게 직렬화/역직렬화할 수 있다.
type JSONField[T any] struct {
	Data T
}

func (j *JSONField[T]) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("JSONField.Scan: unsupported type %T", src)
	}
	return json.Unmarshal(data, &j.Data)
}

func (j JSONField[T]) Value() (driver.Value, error) {
	return json.Marshal(j.Data)
}

func (j JSONField[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Data)
}

func (j *JSONField[T]) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &j.Data)
}

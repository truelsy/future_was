package model

import "time"

// AddServerTime 관리자 페이지에서 설정한 서버 시간 오프셋 이력.
// 가장 마지막 행의 AddSecond 가 현재 적용 오프셋(초, 절대값).
type AddServerTime struct {
	Idx              uint64    `db:"idx" json:"idx"`
	AddSecond        int64     `db:"add_second" json:"add_second"`
	LastEditUserName string    `db:"last_edit_user_name" json:"last_edit_user_name"`
	InsertTime       time.Time `db:"insert_time" json:"insert_time"`
	UpdateTime       time.Time `db:"update_time" json:"update_time"`
}

func (*AddServerTime) TableName() string        { return "TB_ADD_SERVER_TIME" }
func (*AddServerTime) PrimaryKey() string       { return "idx" }
func (*AddServerTime) IsSingleton() bool        { return false }
func (a *AddServerTime) SetPrimaryKey(id int64) { a.Idx = uint64(id) }

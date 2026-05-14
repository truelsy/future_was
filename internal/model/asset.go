package model

import "time"

type Asset struct {
	Idx              uint64    `db:"idx" json:"idx"`
	UserID           uint64    `db:"user_id" json:"user_id"`
	AssetID          uint32    `db:"asset_id" json:"asset_id"`
	Quantity         int64     `db:"quantity" json:"quantity"`
	LastRechargeTime int64     `db:"last_recharge_time" json:"last_recharge_time"` // Unix sec. 0 = 충전 재화 아님.
	InsertTime       time.Time `db:"insert_time" json:"insert_time"`
	UpdateTime       time.Time `db:"update_time" json:"update_time"`
}

func (*Asset) TableName() string        { return "TB_ASSET" }
func (*Asset) PrimaryKey() string       { return "idx" }
func (*Asset) IsSingleton() bool        { return false }
func (a *Asset) SetPrimaryKey(id int64) { a.Idx = uint64(id) }

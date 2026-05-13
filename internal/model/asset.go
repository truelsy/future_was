package model

import "time"

type Asset struct {
	Idx        uint64    `db:"idx" json:"idx"`
	UserID     uint64    `db:"user_id" json:"user_id"`
	AssetID    uint32    `db:"asset_id" json:"asset_id"`
	Quantity   int64     `db:"quantity" json:"quantity"`
	InsertTime time.Time `db:"insert_time,auto" json:"insert_time"`
	UpdateTime time.Time `db:"update_time,auto" json:"update_time"`
}

func (*Asset) TableName() string        { return "TB_ASSET" }
func (*Asset) PrimaryKey() string       { return "idx" }
func (*Asset) IsSingleton() bool        { return false }
func (a *Asset) SetPrimaryKey(id int64) { a.Idx = uint64(id) }

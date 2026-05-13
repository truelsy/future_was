package model

import "time"

type Item struct {
	Idx        uint64    `db:"idx" json:"idx"`
	UserID     uint64    `db:"user_id" json:"user_id"`
	ItemType   uint16    `db:"item_type" json:"item_type"`
	ItemID     uint32    `db:"item_id" json:"item_id"`
	Amount     uint64    `db:"amount" json:"amount"`
	InsertTime time.Time `db:"insert_time,auto" json:"insert_time"`
	UpdateTime time.Time `db:"update_time,auto" json:"update_time"`
}

func (*Item) TableName() string        { return "TB_ITEM" }
func (*Item) PrimaryKey() string       { return "idx" }
func (*Item) IsSingleton() bool        { return false }
func (i *Item) SetPrimaryKey(id int64) { i.Idx = uint64(id) }

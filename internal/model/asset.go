package model

type Asset struct {
	Idx        uint64 `db:"idx" json:"idx"`
	UserID     uint64 `db:"user_id" json:"user_id"`
	AssetID    uint32 `db:"asset_id" json:"asset_id"`
	Quantity   int64  `db:"quantity" json:"quantity"`
	InsertTime uint32 `db:"insert_time" json:"insert_time"`
	UpdateTime uint32 `db:"update_time" json:"update_time"`
}

func (*Asset) TableName() string        { return "TB_ASSET" }
func (*Asset) PrimaryKey() string       { return "idx" }
func (*Asset) IsSingleton() bool        { return false }
func (a *Asset) SetPrimaryKey(id int64) { a.Idx = uint64(id) }

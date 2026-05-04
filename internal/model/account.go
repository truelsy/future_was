package model

type Account struct {
	UserID     uint64 `db:"user_id" json:"user_id"`
	ChannelUID uint64 `db:"channel_uid" json:"channel_uid"`
	DeviceID   string `db:"device_id" json:"device_id"`
	IsActive   uint8  `db:"is_active" json:"is_active"`
	DBShardID  int8   `db:"db_shard_id" json:"db_shard_id"`
	TableID    int8   `db:"table_id" json:"table_id"`
	InsertTime uint32 `db:"insert_time" json:"insert_time"`
	UpdateTime uint32 `db:"update_time" json:"update_time"`
}

func (*Account) TableName() string        { return "TB_ACCOUNT" }
func (*Account) PrimaryKey() string       { return "user_id" }
func (*Account) IsSingleton() bool        { return true }
func (a *Account) SetPrimaryKey(id int64) { a.UserID = uint64(id) }

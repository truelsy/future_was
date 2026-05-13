package model

import "time"

type ServerMaintenance struct {
	Idx                uint64    `db:"idx" json:"idx"`
	ServerType         uint8     `db:"server_type" json:"server_type"`
	StartTime          uint32    `db:"start_time" json:"start_time"`
	EndTime            uint32    `db:"end_time" json:"end_time"`
	Msg                string    `db:"msg" json:"msg"`
	MaintenanceVersion uint32    `db:"maintenance_version" json:"maintenance_version"`
	UseFlag            uint8     `db:"use_flag" json:"use_flag"`
	LastModifier       string    `db:"last_modifier" json:"last_modifier"`
	Description        string    `db:"description" json:"description"`
	InsertTime         time.Time `db:"insert_time,auto" json:"insert_time"`
	UpdateTime         time.Time `db:"update_time,auto" json:"update_time"`
}

func (*ServerMaintenance) TableName() string        { return "TB_SERVER_MAINTENANCE" }
func (*ServerMaintenance) PrimaryKey() string       { return "idx" }
func (*ServerMaintenance) IsSingleton() bool        { return false }
func (m *ServerMaintenance) SetPrimaryKey(id int64) { m.Idx = uint64(id) }

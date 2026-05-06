package model

// Version TB_VERSION 매핑. 클라이언트/서버 버전 활성 정보를 보관한다.
type Version struct {
	Idx             uint64 `db:"idx" json:"idx"`
	ClientVersion   string `db:"client_version" json:"client_version"`
	ServerVersion   string `db:"server_version" json:"server_version"`
	AppID           string `db:"app_id" json:"app_id"`
	IsActive        uint8  `db:"is_active" json:"is_active"`
	UpdateFlag      uint8  `db:"update_flag" json:"update_flag"`
	InspectionFlag  uint8  `db:"inspection_flag" json:"inspection_flag"`
	CatalogFilename string `db:"catalog_filename" json:"catalog_filename"`
	Comment         string `db:"comment" json:"comment"`
	InsertTime      uint32 `db:"insert_time" json:"insert_time"`
	UpdateTime      uint32 `db:"update_time" json:"update_time"`
}

func (*Version) TableName() string         { return "TB_VERSION" }
func (*Version) PrimaryKey() string        { return "idx" }
func (*Version) IsSingleton() bool         { return false }
func (v *Version) SetPrimaryKey(id int64)  { v.Idx = uint64(id) }

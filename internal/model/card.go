package model

import "time"

type Card struct {
	Idx                   uint64                  `db:"idx" json:"idx"`
	UserID                uint64                  `db:"user_id" json:"user_id"`
	CardID                uint32                  `db:"card_id" json:"card_id"`
	TwoWayIdx             uint64                  `db:"two_way_idx" json:"two_way_idx"`
	LevelBreakStep        uint16                  `db:"level_break_step" json:"level_break_step"`
	LevelBreakCurMaterial uint16                  `db:"level_break_cur_material" json:"level_break_cur_material"`
	LimitBreakLevel       uint16                  `db:"limit_break_level" json:"limit_break_level"`
	LimitBreakExp         uint32                  `db:"limit_break_exp" json:"limit_break_exp"`
	Level                 uint16                  `db:"level" json:"level"`
	Exp                   uint32                  `db:"exp" json:"exp"`
	ExpExceed             uint32                  `db:"exp_exceed" json:"exp_exceed"`
	ThemeID               uint16                  `db:"theme_id" json:"theme_id"`
	ExtraThemeID          uint16                  `db:"extra_theme_id" json:"extra_theme_id"`
	IsLock                uint8                   `db:"is_lock" json:"is_lock"`
	Skill                 JSONField[[]*SkillData] `db:"skill" json:"skill"`
	PotentialList         JSONField[PotentialMap] `db:"potential_list" json:"potential_list"`
	EnhanceCardIdx        uint64                  `db:"enhance_card_idx" json:"enhance_card_idx"`
	InsertTime            time.Time               `db:"insert_time" json:"insert_time"`
	UpdateTime            time.Time               `db:"update_time" json:"update_time"`
}

func (*Card) TableName() string        { return "TB_CARD" }
func (*Card) PrimaryKey() string       { return "idx" }
func (*Card) IsSingleton() bool        { return false }
func (c *Card) SetPrimaryKey(id int64) { c.Idx = uint64(id) }

// SkillData 카드 스킬의 도메인 표현.
// model이 protobuf에 의존하지 않도록 pb.SkillData를 미러링한다.
// JSON 필드명은 pb.SkillData와 동일하게 유지하여 DB JSON 컬럼 호환성 보존.
// handler/card/convert.go가 pb.SkillData와의 양방향 변환을 담당한다.
type SkillData struct {
	Exp     uint32 `json:"exp,omitempty"`
	Slot    uint32 `json:"slot,omitempty"`
	Level   uint32 `json:"level,omitempty"`
	SkillID uint32 `json:"skill_id,omitempty"`
}

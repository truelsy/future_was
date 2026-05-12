package model

type Card struct {
	Idx                     uint64                     `db:"idx" json:"idx"`
	UserID                  uint64                     `db:"user_id" json:"user_id"`
	CardID                  uint32                     `db:"card_id" json:"card_id"`
	TwoWayIdx               uint64                     `db:"two_way_idx" json:"two_way_idx"`
	LevelBreakStep          uint16                     `db:"level_break_step" json:"level_break_step"`
	LevelBreakCurMaterial   uint16                     `db:"level_break_cur_material" json:"level_break_cur_material"`
	LimitBreakLevel         uint16                     `db:"limit_break_level" json:"limit_break_level"`
	LimitBreakExp           uint32                     `db:"limit_break_exp" json:"limit_break_exp"`
	LimitBreakExp2          uint32                     `db:"limit_break_exp_2" json:"limit_break_exp_2"`
	Level                   uint16                     `db:"level" json:"level"`
	Exp                     uint32                     `db:"exp" json:"exp"`
	ExpExceed               uint32                     `db:"exp_exceed" json:"exp_exceed"`
	ThemeID                 uint16                     `db:"theme_id" json:"theme_id"`
	ExtraThemeID            uint16                     `db:"extra_theme_id" json:"extra_theme_id"`
	IsLock                  uint8                      `db:"is_lock" json:"is_lock"`
	Skill                   JSONField[[]*SkillData]    `db:"skill" json:"skill"`
	PotentialList           JSONField[any]             `db:"potential_list" json:"potential_list"`
	PotentialExtraLevelList JSONField[any]             `db:"potential_extra_level_list" json:"potential_extra_level_list"`
	SpecialTrainingList     JSONField[any]             `db:"special_training_list" json:"special_training_list"`
	EditionTraining         JSONField[any]             `db:"edition_training" json:"edition_training"`
	EnhanceCardIdx          uint64                     `db:"enhance_card_idx" json:"enhance_card_idx"`
	MaxOpenedExSlot         uint8                      `db:"max_opened_ex_slot" json:"max_opened_ex_slot"`
	ExSlotIdx               uint16                     `db:"ex_slot_idx" json:"ex_slot_idx"`
	ExSlotList              JSONField[any]             `db:"ex_slot_list" json:"ex_slot_list"`
	PowerGradeInfo          JSONField[any]             `db:"power_grade_info" json:"power_grade_info"`
	CardNumber              uint32                     `db:"card_number" json:"card_number"`
	AdditionalData          JSONField[any]             `db:"additional_data" json:"additional_data"`
	RisingPicked            uint8                      `db:"rising_picked" json:"rising_picked"`
	InsertTime              uint32                     `db:"insert_time" json:"insert_time"`
	UpdateTime              uint32                     `db:"update_time" json:"update_time"`
}

func (*Card) TableName() string        { return "TB_CARD" }
func (*Card) PrimaryKey() string       { return "idx" }
func (*Card) IsSingleton() bool        { return false }
func (c *Card) SetPrimaryKey(id int64) { c.Idx = uint64(id) }

package card

import (
	"encoding/json"

	"future_was/internal/model"
	"future_was/pb"
)

// toCardData model.Card를 pb.CardData로 변환한다.
func toCardData(card *model.Card) *pb.CardData {
	return &pb.CardData{
		Idx:                     card.Idx,
		UserId:                  card.UserID,
		CardId:                  card.CardID,
		TwoWayIdx:               card.TwoWayIdx,
		LevelBreakStep:          uint32(card.LevelBreakStep),
		LevelBreakCurMaterial:   uint32(card.LevelBreakCurMaterial),
		LimitBreakLevel:         uint32(card.LimitBreakLevel),
		LimitBreakExp:           card.LimitBreakExp,
		LimitBreakExp_2:         card.LimitBreakExp2,
		Level:                   uint32(card.Level),
		Exp:                     card.Exp,
		ExpExceed:               card.ExpExceed,
		ThemeId:                 uint32(card.ThemeID),
		ExtraThemeId:            uint32(card.ExtraThemeID),
		IsLock:                  uint32(card.IsLock),
		Skill:                   toPbSkills(card.Skill.Data),
		PotentialList:           card.PotentialList.Data,
		PotentialExtraLevelList: jsonBytes(card.PotentialExtraLevelList),
		SpecialTrainingList:     jsonBytes(card.SpecialTrainingList),
		EditionTraining:         jsonBytes(card.EditionTraining),
		EnhanceCardIdx:          card.EnhanceCardIdx,
		MaxOpenedExSlot:         uint32(card.MaxOpenedExSlot),
		ExSlotIdx:               uint32(card.ExSlotIdx),
		ExSlotList:              jsonBytes(card.ExSlotList),
		PowerGradeInfo:          jsonBytes(card.PowerGradeInfo),
		CardNumber:              card.CardNumber,
		AdditionalData:          jsonBytes(card.AdditionalData),
		RisingPicked:            uint32(card.RisingPicked),
		InsertTime:              card.InsertTime,
		UpdateTime:              card.UpdateTime,
	}
}

func jsonBytes(f model.JSONField[any]) []byte {
	b, _ := json.Marshal(f.Data)
	return b
}

// toPbSkills model.SkillData 슬라이스를 pb.SkillData 슬라이스로 변환한다.
// model이 protobuf에 의존하지 않도록 변환 책임을 handler 계층에 둔다.
func toPbSkills(src []*model.SkillData) []*pb.SkillData {
	if len(src) == 0 {
		return nil
	}
	out := make([]*pb.SkillData, 0, len(src))
	for _, s := range src {
		if s == nil {
			continue
		}
		out = append(out, &pb.SkillData{
			Exp:     s.Exp,
			Slot:    s.Slot,
			Level:   s.Level,
			SkillId: s.SkillID,
		})
	}
	return out
}

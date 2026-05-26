package service

import (
	"future_cpbl_web_server/internal/clock"
	"future_cpbl_web_server/internal/model"
	"future_cpbl_web_server/internal/uow"
)

type CardService struct{}

func NewCardService() *CardService {
	return &CardService{}
}

// GetCards 유저의 모든 카드를 반환한다 (UoW를 통한 지연 로딩).
func (s *CardService) GetCards(u *uow.UnitOfWork) ([]*model.Card, error) {
	return u.Cards()
}

// GetCard 지정된 cardIdx의 카드를 반환한다.
// store에 포인터가 저장되므로, 반환된 포인터로 수정하면 store 스냅샷에 반영된다.
func (s *CardService) GetCard(u *uow.UnitOfWork, cardIdx uint64) (*model.Card, error) {
	cards, err := u.Cards()
	if err != nil {
		return nil, err
	}

	for _, card := range cards {
		if card.Idx == cardIdx {
			return card, nil
		}
	}
	return nil, nil
}

// BuildCard 유저가 새로운 카드를 생성한다.
func (s *CardService) BuildCard(userId uint64, cardId uint32) (*model.Card, error) {
	now := clock.Now()
	card := &model.Card{
		Idx:                   0,
		UserID:                userId,
		CardID:                cardId,
		TwoWayIdx:             0,
		LevelBreakStep:        0,
		LevelBreakCurMaterial: 0,
		LimitBreakLevel:       0,
		LimitBreakExp:         0,
		Level:                 1,
		Exp:                   0,
		ExpExceed:             0,
		ThemeID:               0,
		ExtraThemeID:          0,
		IsLock:                0,
		Skill:                 model.JSONField[[]*model.SkillData]{},
		PotentialList:         model.JSONField[model.PotentialMap]{},
		EnhanceCardIdx:        0,
		InsertTime:            now,
		UpdateTime:            now,
	}

	// todo: 카드의 기본 정보를 채워야 한다.
	skills := make([]*model.SkillData, 0)
	skills = append(skills, &model.SkillData{
		Exp:     10,
		Slot:    0,
		Level:   1,
		SkillID: 1,
	})
	skills = append(skills, &model.SkillData{
		Exp:     10,
		Slot:    1,
		Level:   1,
		SkillID: 2,
	})
	card.Skill.Data = skills

	return card, nil
}

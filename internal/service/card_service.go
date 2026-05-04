package service

import (
	"future_next_baseball/internal/model"
	"future_next_baseball/internal/uow"
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

package service

import (
	"time"

	"future_was/internal/errcode"
	"future_was/internal/model"
	"future_was/internal/uow"
)

type ItemService struct{}

func NewItemService() *ItemService {
	return &ItemService{}
}

// GetItems 지정된 유닛 워크를 통해 아이템 목록을 가져온다.
func (s *ItemService) GetItems(u *uow.UnitOfWork) ([]*model.Item, error) {
	return u.Items()
}

// GetItem 지정된 ItemID로 아이템 정보를 가져온다.
func (s *ItemService) GetItem(u *uow.UnitOfWork, itemID uint32) (*model.Item, error) {
	items, err := u.Items()
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.ItemID == itemID {
			return item, nil
		}
	}
	return nil, nil
}

// AddItem 아이템 생성 및 수량 증가 처리
func (s *ItemService) AddItem(u *uow.UnitOfWork, itemID uint32, amount uint64) error {
	item, err := s.GetItem(u, itemID)
	if err != nil {
		return err
	}

	now := time.Now()

	if item != nil {
		item.Amount += amount
		item.UpdateTime = now
		uow.Update(u, item, u.ShardDB())
		return nil
	}

	// 기획 데이터 조회
	designItem, ok := u.Catalog().Item().Find(int32(itemID))
	if !ok {
		return errcode.Newf(errcode.CodeNotFoundDesign, "not found design item. item_id(%v)", itemID)
	}

	// 없으면 생성
	item = &model.Item{
		UserID:     u.UserID(),
		ItemType:   uint16(designItem.ItemType),
		ItemID:     itemID,
		Amount:     amount,
		InsertTime: now,
		UpdateTime: now,
	}
	uow.Create(u, uow.FieldItems, item, u.ShardDB())

	return nil
}

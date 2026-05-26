package service

import (
	"future_cpbl_web_server/internal/clock"
	"future_cpbl_web_server/internal/errcode"
	"future_cpbl_web_server/internal/model"
	"future_cpbl_web_server/internal/uow"
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

	now := clock.Now()

	if item != nil {
		item.Amount += amount
		item.UpdateTime = now
		uow.Update(u, uow.EntityItems, item)
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
	uow.Create(u, uow.EntityItems, item)

	return nil
}

// ConsumeItem 아이템 소비 처리
func (s *ItemService) ConsumeItem(u *uow.UnitOfWork, itemID uint32, amount uint64) error {
	item, err := s.GetItem(u, itemID)
	if err != nil {
		return err
	}

	if item == nil {
		return errcode.Newf(errcode.CodeItemNotFound, "not found item. item_id(%v)", itemID)
	}

	if item.Amount < amount {
		return errcode.Newf(errcode.CodeItemInsufficient, "not enough item. item_id(%v), amount(%v)", itemID, amount)
	}

	item.Amount -= amount
	item.UpdateTime = clock.Now()
	uow.Update(u, uow.EntityItems, item)
	return nil
}

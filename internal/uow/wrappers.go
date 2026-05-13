package uow

import (
	"future_was/internal/model"
)

// Account 계정 정보 로딩한다.
func (u *UnitOfWork) Account() (*model.Account, error) {
	return LoadOne[*model.Account](u, OwnerUser, FieldAccount, u.c.GameDB)
}

// Assets Account.DBShardID로 결정된 shard DB에서 Asset을 로딩한다.
func (u *UnitOfWork) Assets() ([]*model.Asset, error) {
	return LoadList[*model.Asset](u, OwnerUser, FieldAssets, u.ShardDB())
}

// Cards Account.DBShardID로 결정된 shard DB에서 카드를 로딩한다.
func (u *UnitOfWork) Cards() ([]*model.Card, error) {
	return LoadList[*model.Card](u, OwnerUser, FieldCards, u.ShardDB())
}

// Items Account.DBShardID로 결정된 shard DB에서 인벤토리를 로딩한다.
func (u *UnitOfWork) Items() ([]*model.Item, error) {
	return LoadList[*model.Item](u, OwnerUser, FieldItems, u.ShardDB())
}

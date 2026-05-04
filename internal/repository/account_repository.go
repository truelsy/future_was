package repository

import (
	"future_next_baseball/internal/database"
	"future_next_baseball/internal/model"
)

type AccountRepository struct {
	db *database.Database
}

func NewAccountRepository(db *database.Database) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) FindByChannelUID(channelUID uint64) (*model.Account, error) {
	var account model.Account
	err := r.db.FindOne(&account, "channel_uid = ? AND is_active > 0", channelUID)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *AccountRepository) FindUserIdByChannelUID(channelUID uint64) (uint64, error) {
	var userID uint64
	err := r.db.RawGet(&userID, "SELECT user_id FROM TB_ACCOUNT WHERE channel_uid = ? AND is_active > 0", channelUID)
	return userID, err
}

func (r *AccountRepository) FindByUserID(userID uint64) (*model.Account, error) {
	var account model.Account
	err := r.db.FindOne(&account, "user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *AccountRepository) Create(account *model.Account) (int64, error) {
	return r.db.Create(account)
}

func (r *AccountRepository) Update(account *model.Account) (int64, error) {
	return r.db.Save(account, "device_id", "update_time")
}

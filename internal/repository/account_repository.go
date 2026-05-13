package repository

import (
	"errors"

	"future_was/internal/database"
	"future_was/internal/model"
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

// FindUserIdByChannelUID channel_uid로 user_id를 조회한다.
// 미존재 시 (0, false, nil)을 반환하여 호출자가 SQL 에러 sentinel에 의존하지 않도록 한다.
func (r *AccountRepository) FindUserIdByChannelUID(channelUID uint64) (uint64, bool, error) {
	var userID uint64
	err := r.db.RawGet(&userID, "SELECT user_id FROM TB_ACCOUNT WHERE channel_uid = ? AND is_active > 0", channelUID)
	if errors.Is(err, database.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return userID, true, nil
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

// PickShard 신규 계정에 할당할 shard ID를 가중치 풀에서 뽑는다.
// 등록된 weight>0 shard가 없으면 ErrNoShardAvailable.
func (r *AccountRepository) PickShard() (int8, error) {
	return database.PickShard()
}

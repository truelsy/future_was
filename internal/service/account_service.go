package service

import (
	"errors"
	"time"

	"future_next_baseball/internal/database"
	"future_next_baseball/internal/model"
	"future_next_baseball/internal/repository"
	"future_next_baseball/internal/uow"
)

type AccountService struct {
	repo *repository.AccountRepository
}

func NewAccountService(repo *repository.AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

// Login channel_uid로 계정을 조회한다. 없으면 즉시 INSERT하여 user_id를 할당한다.
// 기존 계정이면 device_id/update_time 갱신을 UoW에 큐잉한다.
func (s *AccountService) Login(u *uow.UnitOfWork, channelUID uint64, deviceID string) (*model.Account, bool, error) {
	userID, err := s.repo.FindUserIdByChannelUID(channelUID)
	u.SetUserID(userID)

	now := uint32(time.Now().Unix())

	if err != nil {
		if !errors.Is(err, database.ErrNoRows) {
			return nil, false, err
		}
		// 신규 계정: 즉시 INSERT하여 user_id를 할당한다.
		account := &model.Account{
			ChannelUID: channelUID,
			DeviceID:   deviceID,
			IsActive:   1,
			DBShardID:  10,
			TableID:    -1,
			InsertTime: now,
			UpdateTime: now,
		}
		if err := uow.CreateNow(u, uow.FieldAccount, account, u.Container().GameDB); err != nil {
			return nil, false, err
		}
		return account, true, nil
	}

	account, err := u.Account()
	if err != nil {
		return nil, false, err
	}

	// 기존 계정: device_id/update_time 갱신 큐잉
	account.DeviceID = deviceID
	account.UpdateTime = now
	uow.Update(u, account, u.Container().GameDB)
	return account, false, nil
}

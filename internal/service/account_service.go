package service

import (
	"time"

	"future_was/internal/model"
	"future_was/internal/repository"
	"future_was/internal/uow"
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
	userID, found, err := s.repo.FindUserIdByChannelUID(channelUID)
	if err != nil {
		return nil, false, err
	}

	now := uint32(time.Now().Unix())

	if !found {
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

	u.SetUserID(userID)

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

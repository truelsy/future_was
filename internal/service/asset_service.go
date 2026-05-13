package service

import (
	"time"

	"future_was/internal/errcode"
	"future_was/internal/model"
	"future_was/internal/uow"
)

type AssetService struct{}

func NewAssetService() *AssetService {
	return &AssetService{}
}

// GetAsset 지정된 assetID의 에셋을 반환한다 (UoW를 통한 지연 로딩).
// store에 포인터가 저장되므로, 반환된 포인터로 수정하면 store 스냅샷에 반영된다.
func (s *AssetService) GetAsset(u *uow.UnitOfWork, assetID uint32) (*model.Asset, error) {
	assets, err := u.Assets()
	if err != nil {
		return nil, err
	}

	for _, asset := range assets {
		if asset.AssetID == assetID {
			return asset, nil
		}
	}
	return nil, nil
}

// AddAsset 에셋 생성 또는 수량 증가를 UoW에 큐잉한다.
func (s *AssetService) AddAsset(u *uow.UnitOfWork, assetID uint32, quantity int64) error {
	asset, err := s.GetAsset(u, assetID)
	if err != nil {
		return err
	}

	now := time.Now()

	if asset == nil {
		newAsset := &model.Asset{
			UserID:     u.UserID(),
			AssetID:    assetID,
			Quantity:   quantity,
			InsertTime: now,
			UpdateTime: now,
		}
		uow.Create(u, uow.FieldAssets, newAsset, u.ShardDB())
		return nil
	}

	asset.Quantity += quantity
	asset.UpdateTime = now
	uow.Update(u, asset, u.ShardDB())
	return nil
}

// ConsumeAsset 에셋 수량 차감을 큐잉한다. 에셋이 존재하지 않거나
// 잔액이 부족하면 에러를 반환한다.
func (s *AssetService) ConsumeAsset(u *uow.UnitOfWork, assetID uint32, quantity int64) error {
	asset, err := s.GetAsset(u, assetID)
	if err != nil {
		return err
	}
	if asset == nil {
		return errcode.Newf(errcode.CodeAssetNotFound, "asset not found: asset_id=%d", assetID)
	}
	if asset.Quantity < quantity {
		return errcode.Newf(errcode.CodeAssetInsufficient, "insufficient asset: have %d, need %d", asset.Quantity, quantity)
	}

	asset.Quantity -= quantity
	asset.UpdateTime = time.Now()
	uow.Update(u, asset, u.ShardDB())
	return nil
}

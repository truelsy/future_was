package repository

import (
	"future_next_baseball/internal/database"
	"future_next_baseball/internal/model"
)

type AssetRepository struct {
	db *database.Database
}

func NewAssetRepository(db *database.Database) *AssetRepository {
	return &AssetRepository{db: db}
}

func (r *AssetRepository) FindByUserID(userID uint64) ([]model.Asset, error) {
	var assets []model.Asset
	err := r.db.FindList(&assets, &model.Asset{}, "user_id = ?", nil, userID)
	if err != nil {
		return nil, err
	}
	return assets, nil
}

func (r *AssetRepository) FindByUserIDAndAssetID(userID uint64, assetID uint32) (*model.Asset, error) {
	var asset model.Asset
	err := r.db.FindOne(&asset, "user_id = ? AND asset_id = ?", userID, assetID)
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func (r *AssetRepository) Create(asset *model.Asset) (int64, error) {
	return r.db.Create(asset)
}

func (r *AssetRepository) UpdateQuantity(asset *model.Asset) (int64, error) {
	return r.db.Save(asset, "quantity", "update_time")
}


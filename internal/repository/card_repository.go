package repository

import (
	"future_next_baseball/internal/database"
	"future_next_baseball/internal/model"
)

type CardRepository struct {
	db *database.Database
}

func NewCardRepository(db *database.Database) *CardRepository {
	return &CardRepository{db: db}
}

func (r *CardRepository) FindByUserID(userID uint64) ([]model.Card, error) {
	var cards []model.Card
	err := r.db.FindList(&cards, &model.Card{}, "user_id = ?", nil, userID)
	if err != nil {
		return nil, err
	}
	return cards, nil
}

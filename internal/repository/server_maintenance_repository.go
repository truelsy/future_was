package repository

import (
	"future_was/internal/database"
	"future_was/internal/model"
)

type ServerMaintenanceRepository struct {
	db *database.Database
}

func NewServerMaintenanceRepository(db *database.Database) *ServerMaintenanceRepository {
	return &ServerMaintenanceRepository{db: db}
}

// FindActive use_flag=1인 점검 정보를 maintenance_version DESC로 반환.
func (r *ServerMaintenanceRepository) FindActive() ([]*model.ServerMaintenance, error) {
	var rows []*model.ServerMaintenance
	err := r.db.FindList(
		&rows,
		&model.ServerMaintenance{},
		"use_flag = 1",
		&database.QueryOption{OrderBy: "maintenance_version DESC"},
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

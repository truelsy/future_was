package repository

import (
	"future_next_baseball/internal/database"
	"future_next_baseball/internal/model"
)

type VersionRepository struct {
	db *database.Database
}

func NewVersionRepository(db *database.Database) *VersionRepository {
	return &VersionRepository{db: db}
}

// FindActiveOrderedByServerVersion is_active=1인 행을 server_version DESC 정렬로 반환한다.
// 디자인 데이터 로딩 시 활성 버전 결정에 사용된다.
func (r *VersionRepository) FindActiveOrderedByServerVersion() ([]*model.Version, error) {
	var rows []*model.Version
	err := r.db.FindList(
		&rows,
		&model.Version{},
		"is_active = 1",
		&database.QueryOption{OrderBy: "server_version DESC"},
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

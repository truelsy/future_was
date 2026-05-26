package repository

import (
	"time"

	"future_cpbl_web_server/internal/database"
	"future_cpbl_web_server/internal/model"
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

// FindAll 전체 행을 idx DESC 정렬로 반환한다 (어드민 페이지 목록용).
func (r *VersionRepository) FindAll() ([]*model.Version, error) {
	var rows []*model.Version
	err := r.db.FindList(
		&rows,
		&model.Version{},
		"1 = 1",
		&database.QueryOption{OrderBy: "idx DESC", Limit: 200},
	)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Create 새 행을 INSERT 한다. PK 는 자동 할당되어 m 에 반영된다.
// insert_time/update_time 은 service 시간 컨벤션을 따라 실시각(time.Now) 으로 채운다.
func (r *VersionRepository) Create(m *model.Version) error {
	now := time.Now()
	m.InsertTime = now
	m.UpdateTime = now
	id, err := r.db.Create(m)
	if err != nil {
		return err
	}
	m.Idx = uint64(id)
	return nil
}

// Delete idx 로 행을 삭제한다. 삭제된 행 수 반환.
// 삭제만 한다 — 실제 catalog 갱신은 호출자가 POST /admin/design/reload 로 trigger.
func (r *VersionRepository) Delete(idx uint64) (int64, error) {
	return r.db.Remove(&model.Version{}, idx)
}

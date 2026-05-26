package repository

import (
	"errors"
	"time"

	"future_cpbl_web_server/internal/database"
	"future_cpbl_web_server/internal/model"
)

type AddServerTimeRepository struct {
	db *database.Database
}

func NewAddServerTimeRepository(db *database.Database) *AddServerTimeRepository {
	return &AddServerTimeRepository{db: db}
}

// FindLatest 가장 마지막에 추가된 행을 반환한다.
// 행이 하나도 없으면 (nil, false, nil).
func (r *AddServerTimeRepository) FindLatest() (*model.AddServerTime, bool, error) {
	var m model.AddServerTime
	err := r.db.RawGet(&m, "SELECT * FROM TB_ADD_SERVER_TIME ORDER BY idx DESC LIMIT 1")
	if errors.Is(err, database.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &m, true, nil
}

// Create 새 행을 INSERT 한다. addSecond 는 *적용할 절대 오프셋(초)*.
// insert_time/update_time 은 운영 메타데이터라 clock 오프셋이 아닌 실시각(time.Now)을 사용한다.
func (r *AddServerTimeRepository) Create(addSecond int64, userName string) error {
	now := time.Now()
	m := &model.AddServerTime{
		AddSecond:        addSecond,
		LastEditUserName: userName,
		InsertTime:       now,
		UpdateTime:       now,
	}
	_, err := r.db.Create(m)
	return err
}

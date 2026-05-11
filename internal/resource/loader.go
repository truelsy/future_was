package resource

import (
	"context"
	"fmt"
	"future_next_baseball/internal/database"
	"future_next_baseball/internal/repository"
)

type Loader struct {
	maintenance *repository.ServerMaintenanceRepository
}

func NewLoader(db *database.Database) *Loader {
	return &Loader{
		maintenance: repository.NewServerMaintenanceRepository(db),
	}
}

func (l *Loader) Load(ctx context.Context) (*Snapshot, error) {
	snapshot := newSnapshot()

	m, err := l.maintenance.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active maintenance: %w", err)
	}
	snapshot.maintenance = m

	return snapshot, nil
}

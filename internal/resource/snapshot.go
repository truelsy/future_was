package resource

import (
	"future_next_baseball/internal/model"
)

type Snapshot struct {
	maintenance []*model.ServerMaintenance
}

func newSnapshot() *Snapshot {
	return &Snapshot{}
}

func (s *Snapshot) GetMaintenance() []*model.ServerMaintenance {
	return s.maintenance
}

func (s *Snapshot) ActiveMaintenance(now uint32) *model.ServerMaintenance {
	for _, m := range s.maintenance {
		if m.StartTime <= now && now < m.EndTime {
			return m
		}
	}
	return nil
}

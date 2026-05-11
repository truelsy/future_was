package resource

import (
	"sync"
)

type Store struct {
	snapshot *Snapshot
	mu       sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		snapshot: newSnapshot(),
	}
}

func (s *Store) Get() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func (s *Store) Replace(snapshot *Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = snapshot
}

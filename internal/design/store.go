package design

import "sync"

// Snapshot 한 버전의 모든 디자인 데이터를 담는 불변 스냅샷.
// store에 저장된 후에는 수정하지 않는다.
type Snapshot struct {
	Version string
	batData map[uint32]*BatDataDesign
}

// Store 활성 + 직전 server_version의 Snapshot 두 개와
// client_version → server_version 매핑을 보관한다.
type Store struct {
	current    *Snapshot
	previous   *Snapshot
	versionMap map[string]string // client_version → server_version
	mu         sync.RWMutex
}

func NewStore() *Store {
	return &Store{versionMap: map[string]string{}}
}

// Get server_version으로 Snapshot을 직접 조회한다 (Admin/디버깅용).
func (s *Store) Get(serverVersion string) *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getLocked(serverVersion)
}

// GetByClientVersion 클라이언트 버전으로 매핑된 Snapshot을 반환한다.
// 매핑이 없거나 해당 server_version의 Snapshot이 없으면 nil.
func (s *Store) GetByClientVersion(clientVersion string) *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sv, ok := s.versionMap[clientVersion]
	if !ok {
		return nil
	}
	return s.getLocked(sv)
}

// getLocked 락이 이미 잡힌 상태에서 server_version으로 Snapshot 조회.
func (s *Store) getLocked(serverVersion string) *Snapshot {
	if s.current != nil && s.current.Version == serverVersion {
		return s.current
	}
	if s.previous != nil && s.previous.Version == serverVersion {
		return s.previous
	}
	return nil
}

// CurrentVersion 활성 server_version. 없으면 빈 문자열.
func (s *Store) CurrentVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return ""
	}
	return s.current.Version
}

// PreviousVersion 직전 server_version. 없으면 빈 문자열.
func (s *Store) PreviousVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.previous == nil {
		return ""
	}
	return s.previous.Version
}

// Promote 새 Snapshot을 current로 승격하고, 기존 current는 previous로 이동한다.
func (s *Store) Promote(snap *Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previous = s.current
	s.current = snap
}

// SetPrevious previous를 직접 설정한다 (재시작 시 초기 로드용).
func (s *Store) SetPrevious(snap *Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previous = snap
}

// SetVersionMap client_version → server_version 매핑을 갱신한다.
func (s *Store) SetVersionMap(m map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.versionMap = m
}

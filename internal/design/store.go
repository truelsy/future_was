package design

import (
	"sync"

	"future_next_baseball/internal/design/schema"
)

// Catalog 한 server_version의 모든 디자인 데이터를 담는 불변 카탈로그.
// 도메인별 Table은 PK로 인덱싱되어 있으며, store에 저장된 후 수정하지 않는다.
//
// 사용 예: catalog.BatData().Get(nid)
//
//	catalog.Currency().Get(itemID)
type Catalog struct {
	Version  string
	batData  *Design[int32, *schema.BatDataDesign]
	currency *Design[int32, *schema.CurrencyListDesign]

	// UseFlag = true인 BatData
	batDataByUseFlag *Design[int32, *schema.BatDataDesign]
}

func NewCatalog(version string) *Catalog {
	return &Catalog{
		Version:          version,
		batData:          NewDesign[int32, *schema.BatDataDesign](),
		currency:         NewDesign[int32, *schema.CurrencyListDesign](),
		batDataByUseFlag: NewDesign[int32, *schema.BatDataDesign](),
	}
}

// 도메인 Design 접근자. 새 도메인 추가 시 (a) 위 필드 + NewCatalog 초기화,
// (b) 여기에 접근자, (c) loader.unmarshalInto에 case 추가.
func (c *Catalog) BatData() *Design[int32, *schema.BatDataDesign]          { return c.batData }
func (c *Catalog) Currency() *Design[int32, *schema.CurrencyListDesign]    { return c.currency }
func (c *Catalog) BatDataByUseFlag() *Design[int32, *schema.BatDataDesign] { return c.batDataByUseFlag }

// Store 활성 client_version → Catalog 매핑을 관리한다.
// 같은 server_version을 공유하는 여러 client_version은 동일 Catalog 포인터를 가리킨다.
type Store struct {
	catalogs map[string]*Catalog // client_version → Catalog
	mu       sync.RWMutex
}

func NewStore() *Store {
	return &Store{catalogs: map[string]*Catalog{}}
}

// GetByClientVersion 클라이언트 버전에 해당하는 Catalog을 반환한다.
// 활성 매핑이 없으면 nil.
func (s *Store) GetByClientVersion(clientVersion string) *Catalog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.catalogs[clientVersion]
}

// Replace 전체 client_version → Catalog 매핑을 원자적으로 교체한다.
// Syncer가 TB_VERSION 갱신 시 새 매핑을 적용할 때 사용한다.
func (s *Store) Replace(m map[string]*Catalog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.catalogs = m
}

// CatalogsByServerVersion 현재 보유한 Catalog을 server_version 기준으로 중복 제거하여 반환한다.
// Syncer가 reload 시 기존 Catalog 재사용 여부를 판단할 때 사용한다.
func (s *Store) CatalogsByServerVersion() map[string]*Catalog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*Catalog, len(s.catalogs))
	for _, c := range s.catalogs {
		out[c.Version] = c
	}
	return out
}

// ActiveServerVersions 현재 활성 server_version 목록 (중복 제거, 순서 무보장).
// 로깅/디버깅용.
func (s *Store) ActiveServerVersions() []string {
	m := s.CatalogsByServerVersion()
	out := make([]string, 0, len(m))
	for sv := range m {
		out = append(out, sv)
	}
	return out
}

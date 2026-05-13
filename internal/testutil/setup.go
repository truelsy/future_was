// Package testutil 핸들러 통합 테스트의 공용 셋업/픽스처를 제공한다.
//
// 사용 예 (각 핸들러 패키지의 TestMain):
//
//	func TestMain(m *testing.M) {
//	    cleanup := testutil.Bootstrap()
//	    code := m.Run()
//	    cleanup()
//	    os.Exit(code)
//	}
//
// Bootstrap은 sync.Once로 보호되어 패키지 프로세스당 한 번만 실행된다.
// (Go test는 패키지별 별도 바이너리이므로 패키지마다 새로 호출됨.)
package testutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"

	"future_was/config"
	"future_was/internal/cache"
	"future_was/internal/container"
	"future_was/internal/database"
	"future_was/internal/design"
)

// designVersion testdata 디렉토리명과 일치해야 한다.
const designVersion = "1.00.00.00"

var (
	bootstrapOnce sync.Once
	ctn           *container.Container
	designSrv     *httptest.Server
)

// Bootstrap config/DB/Redis/Design 을 한 번만 초기화한다.
// 반환된 cleanup은 TestMain의 m.Run() 이후에 호출해야 한다.
// 인프라(MySQL/Redis)가 로컬에 떠 있어야 하며, 없으면 panic.
func Bootstrap() (cleanup func()) {
	bootstrapOnce.Do(initOnce)
	return shutdown
}

func initOnce() {
	// 패키지 파일 위치 기준으로 repo root와 testdata 경로 해석.
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)
	repoRoot := filepath.Join(pkgDir, "..", "..")

	cfg, err := config.Load(filepath.Join(repoRoot, "config.yaml"))
	if err != nil {
		panic(fmt.Errorf("testutil: config load: %w", err))
	}

	for _, db := range cfg.Databases {
		d, err := database.Init(db.Name, db.DSN())
		if err != nil {
			panic(fmt.Errorf("testutil: database init [%s]: %w", db.Name, err))
		}
		database.RegisterShard(db.ShardID, d, db.Weight)
	}
	for _, rc := range cfg.Redis {
		if err := cache.Init(rc.Name, rc.Host, rc.Port, rc.Password, rc.DB); err != nil {
			panic(fmt.Errorf("testutil: redis init [%s]: %w", rc.Name, err))
		}
	}

	// testdata 디렉토리를 httptest로 정적 서빙 → Loader가 CDN 대신 이 서버에서 fetch.
	testdataDir := filepath.Join(pkgDir, "testdata", "design")
	designSrv = httptest.NewServer(http.FileServer(http.Dir(testdataDir)))

	loader := design.NewLoader(designSrv.URL, 5)
	cat, err := loader.Load(context.Background(), designVersion)
	if err != nil {
		designSrv.Close()
		panic(fmt.Errorf("testutil: design load: %w", err))
	}
	store := design.NewStore()
	store.Replace(map[string]*design.Catalog{designVersion: cat})

	ctn = container.New(store, nil, nil, nil)

	// 모든 핸들러 테스트가 공유할 기본 계정 — TestAccount() 로 접근.
	testAccount = CreateTestAccount()
}

func shutdown() {
	if designSrv != nil {
		designSrv.Close()
	}
	cache.CloseAll()
	database.CloseAll()
}

// Container Bootstrap 후의 공용 Container 반환.
// 호출 전 Bootstrap을 반드시 부른 상태여야 한다.
func Container() *container.Container { return ctn }

// DesignVersion testdata에 포함된 디자인 버전.
func DesignVersion() string { return designVersion }

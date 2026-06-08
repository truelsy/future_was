package testutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"future_was/internal/database"
	"future_was/internal/handler"
	"future_was/internal/model"
	"future_was/internal/repository"
	"future_was/internal/service"
	"future_was/internal/uow"

	"github.com/labstack/echo/v4"
)

// NewCtxWithUoW userID 기준 echo.Context + UoW 를 만든다.
// 실제 dispatch가 하는 일을 흉내내되 catalog/Container는 패키지 글로벌 사용.
func NewCtxWithUoW(userID uint64) echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	ctn := Container()
	u := uow.New(ctn, userID, ctn.DesignStore.GetByClientVersion(DesignVersion()))
	handler.SetUoW(c, u)
	return c
}

// testAccount Bootstrap 시 한 번 만들어 모든 핸들러 테스트에서 공유한다.
// TestAccount() 통해 접근.
var testAccount *model.Account

// TestAccount Bootstrap 후 사용 가능한 공용 테스트 계정을 반환한다.
// 모든 핸들러 테스트(card/asset/...) 가 동일한 user_id 를 공유하므로
// 도메인별 fixture(테스트 카드/에셋 등) 를 이 위에 얹기만 하면 된다.
// 격리가 필요한 테스트는 CreateTestAccount 를 직접 호출.
func TestAccount() *model.Account { return testAccount }

// CreateTestAccount 매 호출마다 새 channel_uid로 신규 계정을 만들고 commit한다.
// 프로덕션의 AccountService.Login 경로를 그대로 타므로 user_id가 즉시 부여되며
// UserCache까지 채워진다. 셋업 실패 시 panic (테스트 setup 단계에서 fatal).
func CreateTestAccount() *model.Account {
	ctn := Container()
	catalog := ctn.DesignStore.GetByClientVersion(DesignVersion())
	u := uow.New(ctn, 0, catalog)

	repo := repository.NewAccountRepository(ctn.GameDB)
	accSvc := service.NewAccountService(repo)

	channelUID := uint64(time.Now().UnixNano())
	account, _, err := accSvc.Login(u, channelUID, "test-dev")
	if err != nil {
		panic(fmt.Errorf("testutil: create test account: %w", err))
	}

	// 초기 재화 지급
	assetSvc := service.NewAssetService()
	if err := assetSvc.AddAsset(u, 10000, 100); err != nil {
		panic(fmt.Errorf("testutil: add test asset: %w", err))
	}

	// 초기 아이템 지급
	itemSvc := service.NewItemService()
	if err := itemSvc.AddItem(u, 25000, 5); err != nil {
		panic(fmt.Errorf("testutil: add test item: %w", err))
	}

	if err := u.Commit(); err != nil {
		panic(fmt.Errorf("testutil: commit test account: %w", err))
	}
	return account
}

// CreateTestCard 지정 Account의 shard DB에 카드 한 장을 INSERT한다.
// JSONField 들은 zero value로 두어도 빈 객체/배열로 직렬화되며
// (PotentialMap·PotentialExtraLevelList의 MarshalJSON), JSONField[any] 필드는
// 빈 map을 명시해 NULL이 들어가지 않도록 한다.
func CreateTestCard(account *model.Account, svc *service.CardService, cardID uint32) *model.Card {
	shardDB := database.GetShard(account.DBShardID)
	if shardDB == nil {
		panic(fmt.Errorf("testutil: shard %d not registered", account.DBShardID))
	}

	card, err := svc.BuildCard(account.UserID, cardID)
	if err != nil {
		panic(fmt.Errorf("testutil: build test card: %w", err))
	}

	id, err := shardDB.Create(card)
	if err != nil {
		panic(fmt.Errorf("testutil: create test card: %w", err))
	}
	card.Idx = uint64(id)
	return card
}

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

// CreateTestAccount 매 호출마다 새 channel_uid로 신규 계정을 만들고 commit한다.
// 프로덕션의 AccountService.Login 경로를 그대로 타므로 user_id가 즉시 부여되며
// UserCache까지 채워진다. 셋업 실패 시 panic (테스트 setup 단계에서 fatal).
func CreateTestAccount() *model.Account {
	ctn := Container()
	catalog := ctn.DesignStore.GetByClientVersion(DesignVersion())
	u := uow.New(ctn, 0, catalog)

	repo := repository.NewAccountRepository(ctn.GameDB)
	svc := service.NewAccountService(repo)

	channelUID := uint64(time.Now().UnixNano())
	account, _, err := svc.Login(u, channelUID, "test-dev")
	if err != nil {
		panic(fmt.Errorf("testutil: create test account: %w", err))
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
func CreateTestCard(account *model.Account, cardID uint32) *model.Card {
	shardDB := database.GetShard(account.DBShardID)
	if shardDB == nil {
		panic(fmt.Errorf("testutil: shard %d not registered", account.DBShardID))
	}

	now := uint32(time.Now().Unix())
	card := &model.Card{
		UserID:     account.UserID,
		CardID:     cardID,
		Level:      1,
		InsertTime: now,
		UpdateTime: now,
	}
	// JSONField[any] 필드들은 nil → "null" 직렬화를 피하기 위해 빈 map으로.
	card.SpecialTrainingList.Data = map[string]any{}
	card.EditionTraining.Data = map[string]any{}
	card.ExSlotList.Data = map[string]any{}
	card.PowerGradeInfo.Data = map[string]any{}
	card.AdditionalData.Data = map[string]any{}

	id, err := shardDB.Create(card)
	if err != nil {
		panic(fmt.Errorf("testutil: create test card: %w", err))
	}
	card.Idx = uint64(id)
	return card
}

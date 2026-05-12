package account

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"future_was/internal/errcode"
	"future_was/internal/handler"
	"future_was/internal/repository"
	"future_was/internal/service"
	"future_was/internal/testutil/integration"
	"future_was/internal/uow"
	"future_was/pb"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

var testHandler *accountHandler

func TestMain(m *testing.M) {
	cleanup := integration.Bootstrap()

	ctn := integration.Container()
	repo := repository.NewAccountRepository(ctn.GameDB)
	testHandler = &accountHandler{
		svc:      service.NewAccountService(repo),
		assetSvc: service.NewAssetService(),
		c:        ctn,
	}

	code := m.Run()
	cleanup()
	os.Exit(code)
}

// newCtxWithUoW Login 호출에 필요한 echo.Context를 만들고 UoW를 주입한다.
// 실제 dispatch가 하는 일을 흉내내되 catalog/Container는 integration 글로벌 사용.
func newCtxWithUoW() echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	ctn := integration.Container()
	u := uow.New(ctn, 0, ctn.DesignStore.GetByClientVersion(integration.DesignVersion()))
	handler.SetUoW(c, u)
	return c
}

func TestLogin_Success(t *testing.T) {
	// 매 실행마다 새 channel_uid → 신규 계정 INSERT 경로.
	ch := uint64(time.Now().UnixNano())
	dev := "test-dev"

	body, err := proto.Marshal(&pb.LoginRequest{ChannelUid: ch, DeviceId: dev})
	require.NoError(t, err)

	c := newCtxWithUoW()
	res, err := testHandler.Login(c, body)
	require.NoError(t, err)

	// dispatch가 하는 commit을 테스트가 대신 호출 → DB INSERT/UPDATE 실제 반영.
	require.NoError(t, handler.UoW(c).Commit())

	lr, ok := res.(*pb.LoginResponse)
	require.True(t, ok)
	assert.NotZero(t, lr.UserId)
	assert.Equal(t, ch, lr.ChannelUid)
	assert.True(t, lr.IsNew)
	assert.NotEmpty(t, lr.SessionToken)

	// 발급된 토큰이 실제 Redis에 들어가 있는지 검증.
	matched, err := integration.Container().UserSession.VerifyAndRefresh(context.Background(), lr.UserId, lr.SessionToken)
	require.NoError(t, err)
	assert.True(t, matched)
}

// channel_uid=0 은 svc 진입 전에 BadRequest. DB/Redis 의존 없음.
func TestLogin_InvalidInput(t *testing.T) {
	body, err := proto.Marshal(&pb.LoginRequest{ChannelUid: 0, DeviceId: "dev"})
	require.NoError(t, err)

	c := newCtxWithUoW()
	res, err := testHandler.Login(c, body)

	require.Nil(t, res)
	var ae *errcode.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, errcode.CodeBadRequest, ae.Code)
}

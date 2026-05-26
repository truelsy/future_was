package account

import (
	"context"
	"os"
	"testing"
	"time"

	"future_cpbl_web_server/internal/errcode"
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/internal/repository"
	"future_cpbl_web_server/internal/service"
	"future_cpbl_web_server/internal/testutil"
	"future_cpbl_web_server/pb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

var testHandler *accountHandler

func TestMain(m *testing.M) {
	cleanup := testutil.Bootstrap()

	ctn := testutil.Container()
	repo := repository.NewAccountRepository(ctn.GameDB)
	testHandler = &accountHandler{
		svc:      service.NewAccountService(repo),
		assetSvc: service.NewAssetService(),
		itemSvc:  service.NewItemService(),
		c:        ctn,
	}

	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestLogin_Success(t *testing.T) {
	// 매 실행마다 새 channel_uid → 신규 계정 INSERT 경로.
	ch := uint64(time.Now().UnixNano())
	dev := "test-dev"

	body, err := proto.Marshal(&pb.LoginRequest{ChannelUid: ch, DeviceId: dev})
	require.NoError(t, err)

	c := testutil.NewCtxWithUoW(0)
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
	matched, err := testutil.Container().UserSession.VerifyAndRefresh(context.Background(), lr.UserId, lr.SessionToken)
	require.NoError(t, err)
	assert.True(t, matched)
}

// channel_uid=0 은 svc 진입 전에 BadRequest. DB/Redis 의존 없음.
func TestLogin_InvalidInput(t *testing.T) {
	body, err := proto.Marshal(&pb.LoginRequest{ChannelUid: 0, DeviceId: "dev"})
	require.NoError(t, err)

	c := testutil.NewCtxWithUoW(0)
	res, err := testHandler.Login(c, body)

	require.Nil(t, res)
	var ae *errcode.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, errcode.CodeBadRequest, ae.Code)
}

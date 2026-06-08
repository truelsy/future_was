package card

import (
	"os"
	"testing"

	"future_was/internal/handler"
	"future_was/internal/model"
	"future_was/internal/service"
	"future_was/internal/testutil"
	"future_was/pb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const testCardID = 100739 // testdata BAT_DATA.json에 포함된 NID

var (
	testHandler *cardHandler
	testCard    *model.Card
)

func TestMain(m *testing.M) {
	cleanup := testutil.Bootstrap()

	ctn := testutil.Container()
	testHandler = &cardHandler{
		svc: service.NewCardService(),
		c:   ctn,
	}

	// 공용 계정은 testutil.SharedAccount() 가 제공한다.
	// card 도메인 한정 fixture(testCard) 만 여기서 만든다.
	testCard = testutil.CreateTestCard(testutil.TestAccount(), testHandler.svc, testCardID)

	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestGetCardList_Success(t *testing.T) {
	body, err := proto.Marshal(&pb.GetCardsRequest{})
	require.NoError(t, err)

	c := testutil.NewCtxWithUoW(testutil.TestAccount().UserID)
	res, err := testHandler.GetCards(c, body)
	require.NoError(t, err)
	require.NoError(t, handler.UoW(c).Commit())

	resp, ok := res.(*pb.GetCardsResponse)
	require.True(t, ok)
	require.Len(t, resp.Cards, 1)
	assert.Equal(t, testCard.Idx, resp.Cards[0].Idx)
	assert.Equal(t, uint32(testCardID), resp.Cards[0].CardId)
}

func TestUpgradeCardLevel_Success(t *testing.T) {
	body, err := proto.Marshal(&pb.UpgradeCardLevelRequest{CardIdx: testCard.Idx})
	require.NoError(t, err)

	c := testutil.NewCtxWithUoW(testutil.TestAccount().UserID)
	beforeLevel := testCard.Level

	res, err := testHandler.UpgradeCardLevel(c, body)
	require.NoError(t, err)
	require.NoError(t, handler.UoW(c).Commit())

	resp, ok := res.(*pb.UpgradeCardLevelResponse)
	require.True(t, ok)
	require.NotNil(t, resp.Card)
	assert.Equal(t, testCard.Idx, resp.Card.Idx)
	assert.Equal(t, int(beforeLevel+1), int(resp.Card.Level))
}

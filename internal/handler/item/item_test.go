package item

import (
	"future_cpbl_web_server/internal/handler"
	"future_cpbl_web_server/internal/service"
	"future_cpbl_web_server/internal/testutil"
	"future_cpbl_web_server/pb"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

var (
	testHandler *itemHandler
)

func TestMain(m *testing.M) {
	cleanup := testutil.Bootstrap()

	ctn := testutil.Container()
	testHandler = &itemHandler{
		svc: service.NewItemService(),
		c:   ctn,
	}

	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestGetItems_Success(t *testing.T) {
	body, err := proto.Marshal(&pb.GetItemsRequest{})
	require.NoError(t, err)

	c := testutil.NewCtxWithUoW(testutil.TestAccount().UserID)
	res, err := testHandler.GetItems(c, body)
	require.NoError(t, err)
	require.NoError(t, handler.UoW(c).Commit())

	resp, ok := res.(*pb.GetItemsResponse)
	require.True(t, ok)
	assert.NotEmpty(t, resp.Items)
}

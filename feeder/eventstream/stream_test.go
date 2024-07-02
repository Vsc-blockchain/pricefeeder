package eventstream

import (
	"bytes"
	"net/url"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/vsc-blockchain/core/app"
	testutilcli "github.com/vsc-blockchain/core/x/common/testutil/cli"
	"github.com/vsc-blockchain/core/x/common/testutil/genesis"
	oracletypes "github.com/vsc-blockchain/core/x/oracle/types"
	"github.com/vsc-blockchain/pricefeeder/utils"
	"google.golang.org/grpc"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     testutilcli.Config
	network *testutilcli.Network

	eventStream  *Stream
	logs         *bytes.Buffer
	oracleClient oracletypes.QueryClient
}

func (s *IntegrationTestSuite) SetupSuite() {
	utils.InitSDKConfig()

	s.cfg = testutilcli.BuildNetworkConfig(genesis.NewTestGenesisState(app.MakeEncodingConfig()))
	network, err := testutilcli.New(
		s.T(),
		s.T().TempDir(),
		s.cfg,
	)
	s.Require().NoError(err)
	s.network = network

	_, err = s.network.WaitForHeight(1)
	require.NoError(s.T(), err)

	val := s.network.Validators[0]
	grpcEndpoint, tmEndpoint := val.AppConfig.GRPC.Address, val.RPCAddress
	u, err := url.Parse(tmEndpoint)
	require.NoError(s.T(), err)
	u.Scheme = "ws"
	u.Path = "/websocket"

	s.logs = new(bytes.Buffer)
	enableTLS := false
	s.eventStream = Dial(
		u.String(),
		grpcEndpoint,
		enableTLS,
		zerolog.New(s.logs))

	conn, err := grpc.Dial(grpcEndpoint, grpc.WithInsecure())
	require.NoError(s.T(), err)
	s.oracleClient = oracletypes.NewQueryClient(conn)
}

func (s *IntegrationTestSuite) TestStreamWorks() {
	select {
	case <-s.eventStream.ParamsUpdate():
	case <-time.After(15 * time.Second):
		s.T().Fatal("params timeout")
	}
	select {
	case <-s.eventStream.VotingPeriodStarted():
	case <-time.After(15 * time.Second):
		s.T().Fatal("vote period timeout")
	}
	<-time.After(10 * time.Second)
	// assert if params don't change, then no updates are sent
	require.Contains(s.T(), s.logs.String(), "skipping params update as they're not different from the old ones")
	// assert new voting period was signaled
	require.Contains(s.T(), s.logs.String(), "signaled new voting period")
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.eventStream.Close()
	s.network.Cleanup()
}

func TestIntegration(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

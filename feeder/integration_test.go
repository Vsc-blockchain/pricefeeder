package feeder_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/vsc-blockchain/core/app"
	"github.com/vsc-blockchain/core/x/common/asset"
	"github.com/vsc-blockchain/core/x/common/denoms"
	testutilcli "github.com/vsc-blockchain/core/x/common/testutil/cli"
	"github.com/vsc-blockchain/core/x/common/testutil/genesis"
	"github.com/vsc-blockchain/pricefeeder/feeder"
	"github.com/vsc-blockchain/pricefeeder/feeder/eventstream"
	"github.com/vsc-blockchain/pricefeeder/feeder/priceposter"
	"github.com/vsc-blockchain/pricefeeder/feeder/priceprovider"
	"github.com/vsc-blockchain/pricefeeder/feeder/priceprovider/sources"
	"github.com/vsc-blockchain/pricefeeder/types"
	"github.com/vsc-blockchain/pricefeeder/utils"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg     testutilcli.Config
	network *testutilcli.Network

	feeder *feeder.Feeder
	logs   *bytes.Buffer
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
	log := zerolog.New(io.MultiWriter(os.Stderr, s.logs)).Level(zerolog.InfoLevel)

	enableTLS := false
	eventStream := eventstream.Dial(u.String(), grpcEndpoint, enableTLS, log)
	priceProvider := priceprovider.NewPriceProvider(sources.Bitfinex, map[asset.Pair]types.Symbol{
		asset.Registry.Pair(denoms.BTC, denoms.NUSD): "tBTCUSD",
		asset.Registry.Pair(denoms.ETH, denoms.NUSD): "tETHUSD",
	}, json.RawMessage{}, log)
	pricePoster := priceposter.Dial(
		grpcEndpoint,
		s.cfg.ChainID,
		enableTLS,
		val.ClientCtx.Keyring, val.ValAddress, val.Address, log)
	s.feeder = feeder.NewFeeder(eventStream, priceProvider, pricePoster, log)
	s.feeder.Run()
}

func (s *IntegrationTestSuite) TestOk() {
	<-time.After(30 * time.Second) // TODO
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.network.Cleanup()
	s.feeder.Close()
}

func TestIntegration(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

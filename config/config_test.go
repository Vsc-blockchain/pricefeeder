package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vsc-blockchain/pricefeeder/utils"
)

func init() {
	app.SetPrefixes(app.AccountAddressPrefix)
}

func TestConfig_Get(t *testing.T) {

	os.Setenv("CHAIN_ID", "vsc-localnet-0")
	os.Setenv("GRPC_ENDPOINT", "localhost:9090")
	os.Setenv("WEBSOCKET_ENDPOINT", "ws://localhost:26657/websocket")
	os.Setenv("FEEDER_MNEMONIC", "earth wash broom grow recall fitness")
	os.Setenv(
		"EXCHANGE_SYMBOLS_MAP",
		"{\"bitfinex\": {\"ubtc:unusd\": \"tBTCUSD\", \"ueth:unusd\": \"tETHUSD\", \"uusd:unusd\": \"tUSTUSD\"}}",
	)

	utils.InitSDKConfig()

	os.Setenv("VALIDATOR_ADDRESS", "nibivaloper1d7zygazerfwx4l362tnpcp0ramzm97xvv9ryxr")
	_, err := Get()
	require.NoError(t, err)
}

func TestConfig_Without_EXCHANGE_SYMBOLS_MAP(t *testing.T) {
	os.Unsetenv("EXCHANGE_SYMBOLS_MAP")
	os.Setenv("CHAIN_ID", "vsc-localnet-0")
	os.Setenv("GRPC_ENDPOINT", "localhost:9090")
	os.Setenv("WEBSOCKET_ENDPOINT", "ws://localhost:26657/websocket")
	os.Setenv("FEEDER_MNEMONIC", "earth wash broom grow recall fitness")
	utils.InitSDKConfig()
	os.Setenv("VALIDATOR_ADDRESS", "nibivaloper1d7zygazerfwx4l362tnpcp0ramzm97xvv9ryxr")
	_, err := Get()
	require.NoError(t, err)
}

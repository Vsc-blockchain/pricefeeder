package sources

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/types"
)

const vscdexTestConfig = `{
	"rpc_url": "https://rpc.vscblockchain.org",
	"pools": {
		"VSGUSD": {
			"address": "0xdbd23389df918176abf0e6f73cadefbbbac93d87",
			"token0_decimals": 6,
			"token1_decimals": 18,
			"invert": true,
			"min_quote_liquidity": 25000
		}
	}
}`

func TestVscdexPriceUpdate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rawPrices, err := VscdexPriceUpdate(json.RawMessage(vscdexTestConfig))(
			set.New[types.Symbol]("VSGUSD"), zerolog.New(io.Discard),
		)
		require.NoError(t, err)
		require.Len(t, rawPrices, 1)
		require.NotZero(t, rawPrices["VSGUSD"])
	})

	t.Run("thin pool is skipped", func(t *testing.T) {
		var c VscdexConfig
		require.NoError(t, json.Unmarshal([]byte(vscdexTestConfig), &c))
		pool := c.Pools["VSGUSD"]
		pool.MinQuoteLiquidity = 1e12 // far above any real depth
		c.Pools["VSGUSD"] = pool

		cfg, err := json.Marshal(c)
		require.NoError(t, err)

		_, err = VscdexPriceUpdate(cfg)(set.New[types.Symbol]("VSGUSD"), zerolog.New(io.Discard))
		require.ErrorContains(t, err, "no usable prices")
	})

	t.Run("config is required", func(t *testing.T) {
		_, err := VscdexPriceUpdate(nil)(set.New[types.Symbol]("VSGUSD"), zerolog.New(io.Discard))
		require.ErrorContains(t, err, "missing vscdex config")
	})
}

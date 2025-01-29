package sources

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/types"
)

func TestAscendexPriceUpdate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rawPrices, err := AscendexPriceUpdate(set.New[types.Symbol]("BTC/USDT", "ETH/USDT"), zerolog.New(io.Discard))
		require.NoError(t, err)
		require.Equal(t, 2, len(rawPrices))
		require.NotZero(t, rawPrices["BTC/USDT"])
		require.NotZero(t, rawPrices["ETH/USDT"])
	})
}

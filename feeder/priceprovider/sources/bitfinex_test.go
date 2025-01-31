package sources

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/types"
)

func TestBitfinexSource(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rawPrices, err := BitfinexPriceUpdate(set.New[types.Symbol]("tBTCUSD", "tETHUSD"), zerolog.New(io.Discard))
		require.NoError(t, err)
		require.Equal(t, 2, len(rawPrices))
		require.NotZero(t, rawPrices["tBTCUSD"])
		require.NotZero(t, rawPrices["tETHUSD"])
	})
}

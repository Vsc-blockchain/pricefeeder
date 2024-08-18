package sources

import (
	"fmt"
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/types"
)

func TestUniswapPriceUpdate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rawPrices, err := UniswapPriceUpdate(set.New[types.Symbol]("VSGUSD"), zerolog.New(io.Discard))
		require.NoError(t, err)
		require.Equal(t, 2, len(rawPrices))
		fmt.Println(rawPrices)
		require.NotZero(t, rawPrices["ETHUSD"])
		require.NotZero(t, rawPrices["VSGUSD"])
	})
}

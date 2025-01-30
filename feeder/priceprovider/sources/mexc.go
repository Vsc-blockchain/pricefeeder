package sources

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/metrics"
	"github.com/vsc-blockchain/pricefeeder/types"
)

const (
	Mexc = "mexc"
)

var _ types.FetchPricesFunc = MexcPriceUpdate

type MexcResponse []struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

// MexcPriceUpdate returns the prices for given symbols or an error.
// Check out the Mexc API under https://mexcdevelop.github.io/apidocs/spot_v3_en/#general-info
func MexcPriceUpdate(symbols set.Set[types.Symbol], logger zerolog.Logger) (rawPrices map[types.Symbol]float64, err error) {
	url := "https://api.mexc.com/api/v3/ticker/price"

	resp, err := http.Get(url)
	if err != nil {
		logger.Err(err).Msg("failed to fetch prices from Mexc")
		metrics.PriceSourceCounter.WithLabelValues(Mexc, "false").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Err(err).Msg("failed to read response body from Mexc")
		metrics.PriceSourceCounter.WithLabelValues(Mexc, "false").Inc()
		return nil, err
	}

	var response MexcResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		logger.Err(err).Msg("failed to unmarshal response body from Mexc")
		metrics.PriceSourceCounter.WithLabelValues(Mexc, "false").Inc()
		return nil, err
	}

	rawPrices = make(map[types.Symbol]float64)

	for _, ticker := range response {
		symbol := types.Symbol(ticker.Symbol)
		price, err := strconv.ParseFloat(ticker.Price, 64)
		if err != nil {
			logger.Err(err).Msgf("failed to parse price for %s on data source %s", symbol, Mexc)
			continue
		}

		if _, ok := symbols[symbol]; ok {
			rawPrices[symbol] = price
		}
	}
	logger.Debug().Msgf("fetched prices for %s on data source %s: %v", symbols, Mexc, rawPrices)
	metrics.PriceSourceCounter.WithLabelValues(Mexc, "true").Inc()
	return rawPrices, nil
}

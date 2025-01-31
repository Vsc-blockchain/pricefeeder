package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/metrics"
	"github.com/vsc-blockchain/pricefeeder/types"
)

const (
	Okex = "okex"
)

var _ types.FetchPricesFunc = OkexPriceUpdate

type OkexTicker struct {
	Symbol string `json:"instId"`
	Price  string `json:"last"`
}

type OkexResponse struct {
	Data []OkexTicker `json:"data"`
}

// OkexPriceUpdate returns the prices for given symbols or an error.
// Uses OKEX API at https://www.okx.com/docs-v5/en/#rest-api-market-data.
func OkexPriceUpdate(symbols set.Set[types.Symbol], logger zerolog.Logger) (rawPrices map[types.Symbol]float64, err error) {
	url := "https://www.okx.com/api/v5/market/tickers?instType=SPOT"

	resp, err := http.Get(url)
	if err != nil {
		logger.Err(err).Msg("failed to fetch prices from Okex")
		metrics.PriceSourceCounter.WithLabelValues(Okex, "false").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Err(err).Msg("failed to read response body from Okex")
		metrics.PriceSourceCounter.WithLabelValues(Okex, "false").Inc()
		return nil, err
	}

	var response OkexResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		logger.Err(err).Msg("failed to unmarshal response body from Okex")
		metrics.PriceSourceCounter.WithLabelValues(Okex, "false").Inc()
		return nil, err
	}

	rawPrices = make(map[types.Symbol]float64)
	for _, ticker := range response.Data {

		symbol := types.Symbol(ticker.Symbol)
		if !symbols.Has(symbol) {
			continue
		}

		price, err := strconv.ParseFloat(ticker.Price, 64)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("failed to parse price for %s on data source %s", symbol, Okex))
			continue
		}

		rawPrices[symbol] = price
		logger.Debug().Msg(fmt.Sprintf("fetched price for %s on data source %s: %f", symbol, Okex, price))
	}

	metrics.PriceSourceCounter.WithLabelValues(Okex, "true").Inc()
	return rawPrices, nil
}

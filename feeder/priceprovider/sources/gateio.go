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
	GateIo = "gateio"
)

var _ types.FetchPricesFunc = GateIoPriceUpdate

// GateIoPriceUpdate returns the prices given the symbols or an error.
// Uses the GateIo API at https://www.gate.io/docs/developers/apiv4/en/#get-details-of-a-specifc-currency-pair.
func GateIoPriceUpdate(symbols set.Set[types.Symbol], logger zerolog.Logger) (rawPrices map[types.Symbol]float64, err error) {
	url := "https://api.gateio.ws/api/v4/spot/tickers"
	resp, err := http.Get(url)
	if err != nil {
		logger.Err(err).Msg("failed to fetch prices from GateIo")
		metrics.PriceSourceCounter.WithLabelValues(GateIo, "false").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Err(err).Msg("failed to read response body from GateIo")
		metrics.PriceSourceCounter.WithLabelValues(GateIo, "false").Inc()
		return nil, err
	}

	var tickers []map[string]interface{}
	err = json.Unmarshal(b, &tickers)
	if err != nil {
		logger.Err(err).Msg("failed to unmarshal response body from GateIo")
		metrics.PriceSourceCounter.WithLabelValues(GateIo, "false").Inc()
		return nil, err
	}

	rawPrices = make(map[types.Symbol]float64)
	for _, ticker := range tickers {
		symbol := types.Symbol(ticker["currency_pair"].(string))
		if !symbols.Has(symbol) {
			continue
		}

		price, err := strconv.ParseFloat(ticker["last"].(string), 64)
		if err != nil {
			logger.Err(err).Msg(fmt.Sprintf("failed to parse price for %s on data source %s", symbol, GateIo))
			continue
		}

		rawPrices[symbol] = price
		logger.Debug().Msg(fmt.Sprintf("fetched price for %s on data source %s: %f", symbol, GateIo, price))
	}

	metrics.PriceSourceCounter.WithLabelValues(GateIo, "true").Inc()
	return rawPrices, nil
}

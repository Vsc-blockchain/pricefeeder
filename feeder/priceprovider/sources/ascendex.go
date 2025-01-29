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
	Ascendex = "ascendex"
)

var _ types.FetchPricesFunc = AscendexPriceUpdate

type AscedexData struct {
	Symbol string   `json:"symbol"`
	Open   string   `json:"open"`
	Close  string   `json:"close"`
	High   string   `json:"high"`
	Low    string   `json:"low"`
	Volume string   `json:"volume"`
	Ask    []string `json:"ask"`
	Bid    []string `json:"bid"`
	Type   string   `json:"type"`
}

type AscendexResponse struct {
	Code int           `json:"code"`
	Data []AscedexData `json:"data"`
}

// AscendexPriceUpdate returns the prices for given symbols or an error.
// Check out the Ascendex API under https://ascendex.github.io/ascendex-pro-api/#ascendex-pro-api-documentation
func AscendexPriceUpdate(symbols set.Set[types.Symbol], logger zerolog.Logger) (rawPrices map[types.Symbol]float64, err error) {
	url := "https://ascendex.com/api/pro/v1/spot/ticker"

	resp, err := http.Get(url)
	if err != nil {
		logger.Err(err).Msg("failed to fetch prices from Ascedex")
		metrics.PriceSourceCounter.WithLabelValues(Ascendex, "false").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Err(err).Msg("failed to read response body from Ascedex")
		metrics.PriceSourceCounter.WithLabelValues(Ascendex, "false").Inc()
		return nil, err
	}

	var response AscendexResponse
	err = json.Unmarshal(b, &response)
	if err != nil {
		logger.Err(err).Msg("failed to unmarshal response body from Ascedex")
		metrics.PriceSourceCounter.WithLabelValues(Ascendex, "false").Inc()
		return nil, err
	}

	rawPrices = make(map[types.Symbol]float64)

	for _, ticker := range response.Data {
		symbol := types.Symbol(ticker.Symbol)
		price, err := strconv.ParseFloat(ticker.Close, 64)
		if err != nil {
			logger.Err(err).Msgf("failed to parse price for %s on data source %s", symbol, Ascendex)
			continue
		}

		if _, ok := symbols[symbol]; ok {
			rawPrices[symbol] = price
		}
	}
	logger.Debug().Msgf("fetched prices for %s on data source %s: %v", symbols, Ascendex, rawPrices)
	metrics.PriceSourceCounter.WithLabelValues(Ascendex, "true").Inc()
	return rawPrices, nil
}

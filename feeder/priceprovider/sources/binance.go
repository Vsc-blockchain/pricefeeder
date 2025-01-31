package sources

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/metrics"
	"github.com/vsc-blockchain/pricefeeder/types"
)

const (
	Binance = "binance"
)

var _ types.FetchPricesFunc = BinancePriceUpdate

type BinanceTicker struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price,string"`
}

func BinanceSymbolCsv(symbols set.Set[types.Symbol]) string {
	s := ""
	for symbol := range symbols {
		s += "%22" + string(symbol) + "%22,"
	}
	// chop off trailing comma
	return s[:len(s)-1]
}

// BinancePriceUpdate returns the prices given the symbols or an error.
// Uses the Binance API at https://docs.binance.us/#price-data.
func BinancePriceUpdate(symbols set.Set[types.Symbol], logger zerolog.Logger) (rawPrices map[types.Symbol]float64, err error) {
	url := "https://api.binance.us/api/v3/ticker/price?symbols=%5B" + BinanceSymbolCsv(symbols) + "%5D"
	resp, err := http.Get(url)
	if err != nil {
		logger.Err(err).Msg("failed to fetch prices from Binance")
		metrics.PriceSourceCounter.WithLabelValues(Binance, "false").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Err(err).Msg("failed to read response body from Binance")
		metrics.PriceSourceCounter.WithLabelValues(Binance, "false").Inc()
		return nil, err
	}

	tickers := make([]BinanceTicker, len(symbols))

	err = json.Unmarshal(b, &tickers)
	if err != nil {
		logger.Err(err).Msg("failed to unmarshal response body from Binance")
		metrics.PriceSourceCounter.WithLabelValues(Binance, "false").Inc()
		return nil, err
	}

	rawPrices = make(map[types.Symbol]float64)
	for _, ticker := range tickers {
		rawPrices[types.Symbol(ticker.Symbol)] = ticker.Price
		logger.Debug().Msgf("fetched price for %s on data source %s: %f", ticker.Symbol, Binance, ticker.Price)
	}
	metrics.PriceSourceCounter.WithLabelValues(Binance, "true").Inc()

	return rawPrices, nil
}

package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/metrics"
	"github.com/vsc-blockchain/pricefeeder/types"
)

const (
	Bitfinex = "bitfinex"
)

var _ types.FetchPricesFunc = BitfinexPriceUpdate

func BitfinexSymbolCsv(symbols set.Set[types.Symbol]) string {
	s := ""
	for symbol := range symbols {
		s += string(symbol) + ","
	}
	return s[:len(s)-1]
}

// BitfinexPriceUpdate returns the prices given the symbols or an error.
func BitfinexPriceUpdate(symbols set.Set[types.Symbol], logger zerolog.Logger) (rawPrices map[types.Symbol]float64, err error) {
	type ticker []interface{}
	// minSize is the number of fields Bitfinex documents for a trading pair
	// ticker. Bitfinex appends new fields over time, so we only require the
	// prefix we actually read.
	const minSize = 11
	const lastPriceIndex = 7
	const symbolNameIndex = 0

	var url string = "https://api-pub.bitfinex.com/v2/tickers?symbols=" + BitfinexSymbolCsv(symbols)
	resp, err := http.Get(url)
	if err != nil {
		logger.Err(err).Msg("failed to fetch prices from Bitfinex")
		metrics.PriceSourceCounter.WithLabelValues(Bitfinex, "false").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Err(err).Msg("failed to read response body from Bitfinex")
		metrics.PriceSourceCounter.WithLabelValues(Bitfinex, "false").Inc()
		return nil, err
	}
	var tickers []ticker

	err = json.Unmarshal(b, &tickers)
	if err != nil {
		logger.Err(err).Msg("failed to unmarshal response body from Bitfinex")
		metrics.PriceSourceCounter.WithLabelValues(Bitfinex, "false").Inc()
		return nil, err
	}

	rawPrices = make(map[types.Symbol]float64)
	for _, ticker := range tickers {
		// one malformed ticker must not take down every other symbol of this source
		if len(ticker) < minSize {
			logger.Err(fmt.Errorf("impossible to parse ticker size %d, %#v", len(ticker), ticker)).Msg("skipping malformed ticker")
			continue
		}
		symbol, ok := ticker[symbolNameIndex].(string)
		if !ok {
			logger.Err(fmt.Errorf("unexpected symbol type in ticker %#v", ticker)).Msg("skipping malformed ticker")
			continue
		}
		lastPrice, ok := ticker[lastPriceIndex].(float64)
		if !ok {
			logger.Err(fmt.Errorf("unexpected price type in ticker %#v", ticker)).Msg("skipping malformed ticker")
			continue
		}

		rawPrices[types.Symbol(symbol)] = lastPrice
		logger.Debug().Msg(fmt.Sprintf("fetched price for %s on data source %s: %f", symbol, Bitfinex, lastPrice))
	}

	metrics.PriceSourceCounter.WithLabelValues(Bitfinex, "true").Inc()

	return rawPrices, nil
}

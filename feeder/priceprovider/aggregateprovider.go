package priceprovider

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/vsc-blockchain/core/x/common/asset"
	"github.com/vsc-blockchain/pricefeeder/metrics"
	"github.com/vsc-blockchain/pricefeeder/types"
)

var _ types.PriceProvider = (*AggregatePriceProvider)(nil)

// AggregatePriceProvider aggregates multiple price providers
// and queries them for prices.
type AggregatePriceProvider struct {
	logger    zerolog.Logger
	providers map[int]types.PriceProvider // we use a map here to provide random ranging (since golang's map range is unordered)
}

// NewAggregatePriceProvider instantiates a new AggregatePriceProvider instance
// given multiple PriceProvider.
func NewAggregatePriceProvider(
	sourcesToPairSymbolMap map[string]map[asset.Pair]types.Symbol,
	sourceConfigMap map[string]json.RawMessage,
	logger zerolog.Logger,
) types.PriceProvider {
	providers := make(map[int]types.PriceProvider, len(sourcesToPairSymbolMap))
	i := 0
	for sourceName, pairToSymbolMap := range sourcesToPairSymbolMap {
		providers[i] = NewPriceProvider(sourceName, pairToSymbolMap, sourceConfigMap[sourceName], logger)
		i++
	}

	return AggregatePriceProvider{
		logger:    logger.With().Str("component", "aggregate-price-provider").Logger(),
		providers: providers,
	}
}

var aggregatePriceProvider = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: metrics.PrometheusNamespace,
	Name:      "aggregate_prices_total",
	Help:      "The total number prices provided by the aggregate price provider, by pair, source, and success status",
}, []string{"pair", "source", "success"})

// GetPrice fetches the first available and correct price from the wrapped PriceProviders.
// Iteration is exhaustive and random.
// If no correct PriceResponse is found, then an invalid PriceResponse is returned.
func (a AggregatePriceProvider) GetPrice(pair asset.Pair) types.Price {
	// iterate randomly, if we find a valid price, we return it
	// otherwise we go onto the next PriceProvider to ask for prices.
	var allPrices []types.Price

	for _, p := range a.providers {
		price := p.GetPrice(pair)
		if price.Valid {
			aggregatePriceProvider.WithLabelValues(pair.String(), price.SourceName, "true").Inc()
			allPrices = append(allPrices, price)
		}
	}

	if len(allPrices) > 0 {
		finalPrice := computeConsolidatedPrice(allPrices, pair)
		return finalPrice
	}

	// if we reach here no valid symbols were found
	a.logger.Warn().Str("pair", pair.String()).Msg("no valid price found")
	aggregatePriceProvider.WithLabelValues(pair.String(), "missing", "false").Inc()
	return types.Price{
		SourceName: "missing",
		Pair:       pair,
		Price:      0,
		Valid:      false,
	}
}

func (a AggregatePriceProvider) Close() {
	for _, p := range a.providers {
		p.Close()
	}
}

// computeConsolidatedPrice computes the consolidated price from the given map of prices.
// it removes outliers and computes the median of the remaining prices.
func computeConsolidatedPrice(prices []types.Price, pair asset.Pair) types.Price {
	if len(prices) == 0 {
		return types.Price{Price: -1, Pair: pair, SourceName: "missing", Valid: false}
	}
	if len(prices) == 1 {
		return prices[0]
	}
	if len(prices) == 2 {
		avg := (prices[0].Price + prices[1].Price) / 2
		return types.Price{Price: avg, Pair: pair, SourceName: "consolidated", Valid: true}
	}

	floatPrices := make([]float64, len(prices))
	for i, p := range prices {
		floatPrices[i] = p.Price
	}

	// remove outliers, then take median
	cleaned := removeOutliers(floatPrices)
	if len(cleaned) == 0 {
		return types.Price{Price: -1, Pair: pair, SourceName: "missing", Valid: false}
	}
	return types.Price{Price: median(cleaned), Pair: pair, SourceName: "consolidated", Valid: true}
}

// removeOutliers removes outliers from the given prices slice.
func removeOutliers(prices []float64) []float64 {
	mean, stddev := meanAndStdDev(prices)
	var filtered []float64
	for _, p := range prices {
		if math.Abs(p-mean) <= 1*stddev {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// median returns the median of the given prices slice.
func median(prices []float64) float64 {
	sort.Float64s(prices)
	mid := len(prices) / 2
	if len(prices)%2 == 1 {
		return prices[mid]
	}
	return (prices[mid-1] + prices[mid]) / 2
}

// meanAndStdDev returns the mean and standard deviation of the given prices slice.
func meanAndStdDev(prices []float64) (float64, float64) {
	var sum float64
	for _, p := range prices {
		sum += p
	}
	mean := sum / float64(len(prices))
	var variance float64
	for _, p := range prices {
		diff := p - mean
		variance += diff * diff
	}
	variance /= float64(len(prices))
	return mean, math.Sqrt(variance)
}

package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/vsc-blockchain/core/x/common/set"
	"github.com/vsc-blockchain/pricefeeder/metrics"
	"github.com/vsc-blockchain/pricefeeder/types"
)

const (
	Vscdex = "vscdex"
)

const uniswapV2PairABIJSON = `[{"constant":true,"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"_reserve0","type":"uint112"},{"internalType":"uint112","name":"_reserve1","type":"uint112"},{"internalType":"uint32","name":"_blockTimestampLast","type":"uint32"}],"payable":false,"stateMutability":"view","type":"function"}]`

// VscdexPool describes a single Uniswap V2 style pair contract to read a price from.
type VscdexPool struct {
	// Address of the pair contract.
	Address string `json:"address"`
	// Token0Decimals and Token1Decimals are the decimals of the pair's tokens,
	// in the order the pair contract reports them via getReserves.
	Token0Decimals int64 `json:"token0_decimals"`
	Token1Decimals int64 `json:"token1_decimals"`
	// Invert reports whether the quote token is token0 rather than token1.
	Invert bool `json:"invert"`
	// MinQuoteLiquidity is the minimum amount of quote token, in whole units,
	// the pool must hold for its price to be considered usable. A pool thinner
	// than this is trivially manipulable, so we skip it and let the feeder
	// abstain rather than vote a price we cannot defend. Must be set.
	MinQuoteLiquidity float64 `json:"min_quote_liquidity"`
}

// VscdexConfig is the source config for the vscdex price source. It is
// deliberately free of defaults: both the RPC endpoint and every pool address
// have to be stated explicitly, so the feeder can never silently read a price
// off a chain or a pool that was not meant to be an oracle source.
type VscdexConfig struct {
	// RpcUrl is the EVM JSON-RPC endpoint of the chain the pools live on.
	RpcUrl string `json:"rpc_url"`
	// Pools maps a symbol, as used in EXCHANGE_SYMBOLS_MAP, to its pool.
	Pools map[types.Symbol]VscdexPool `json:"pools"`
}

// VscdexPriceUpdate returns the prices of the given symbols, read from the
// Uniswap V2 style pools configured for this source.
func VscdexPriceUpdate(sourceConfig json.RawMessage) types.FetchPricesFunc {
	return func(symbols set.Set[types.Symbol], logger zerolog.Logger) (map[types.Symbol]float64, error) {
		c, err := extractVscdexConfig(sourceConfig)
		if err != nil {
			logger.Err(err).Msg("failed to extract vscdex config")
			metrics.PriceSourceCounter.WithLabelValues(Vscdex, "false").Inc()
			return nil, err
		}

		client, err := ethclient.Dial(c.RpcUrl)
		if err != nil {
			logger.Err(err).Msg("failed to connect to the vscdex EVM client")
			metrics.PriceSourceCounter.WithLabelValues(Vscdex, "false").Inc()
			return nil, err
		}
		defer client.Close()

		parsedABI, err := abi.JSON(strings.NewReader(uniswapV2PairABIJSON))
		if err != nil {
			logger.Err(err).Msg("failed to parse contract ABI")
			metrics.PriceSourceCounter.WithLabelValues(Vscdex, "false").Inc()
			return nil, err
		}

		rawPrices := make(map[types.Symbol]float64)
		for symbol := range symbols {
			pool, ok := c.Pools[symbol]
			if !ok {
				logger.Warn().Str("symbol", string(symbol)).Msg("no vscdex pool configured for symbol")
				continue
			}

			price, quoteLiquidity, err := poolPrice(client, parsedABI, pool)
			if err != nil {
				logger.Err(err).Str("symbol", string(symbol)).Msg("failed to fetch price from vscdex pool")
				continue
			}

			if quoteLiquidity < pool.MinQuoteLiquidity {
				logger.Warn().
					Str("symbol", string(symbol)).
					Float64("quote-liquidity", quoteLiquidity).
					Float64("min-quote-liquidity", pool.MinQuoteLiquidity).
					Msg("vscdex pool below minimum liquidity, skipping")
				continue
			}

			rawPrices[symbol] = price
			logger.Debug().Msgf("fetched price for %s on data source %s: %f", symbol, Vscdex, price)
		}

		if len(rawPrices) == 0 {
			metrics.PriceSourceCounter.WithLabelValues(Vscdex, "false").Inc()
			return nil, fmt.Errorf("no usable prices from vscdex")
		}

		metrics.PriceSourceCounter.WithLabelValues(Vscdex, "true").Inc()
		return rawPrices, nil
	}
}

func extractVscdexConfig(sourceConfig json.RawMessage) (VscdexConfig, error) {
	var c VscdexConfig
	if len(sourceConfig) == 0 {
		return c, fmt.Errorf("missing vscdex config: set it in DATASOURCE_CONFIG_MAP")
	}
	if err := json.Unmarshal(sourceConfig, &c); err != nil {
		return c, fmt.Errorf("failed to parse vscdex config: %w", err)
	}
	if c.RpcUrl == "" {
		return c, fmt.Errorf("vscdex config: rpc_url is required")
	}
	if len(c.Pools) == 0 {
		return c, fmt.Errorf("vscdex config: at least one pool is required")
	}
	for symbol, pool := range c.Pools {
		if !common.IsHexAddress(pool.Address) {
			return c, fmt.Errorf("vscdex config: pool %s has invalid address %q", symbol, pool.Address)
		}
		if pool.Token0Decimals <= 0 || pool.Token1Decimals <= 0 {
			return c, fmt.Errorf("vscdex config: pool %s needs token0_decimals and token1_decimals", symbol)
		}
		if pool.MinQuoteLiquidity <= 0 {
			return c, fmt.Errorf("vscdex config: pool %s needs a positive min_quote_liquidity", symbol)
		}
	}
	return c, nil
}

// poolPrice returns the price of the pool's base token denominated in its quote
// token, along with the amount of quote token held by the pool.
func poolPrice(client *ethclient.Client, parsedABI abi.ABI, pool VscdexPool) (price float64, quoteLiquidity float64, err error) {
	addr := common.HexToAddress(pool.Address)
	callData, err := parsedABI.Pack("getReserves")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to pack call data: %w", err)
	}

	result, err := client.CallContract(context.Background(), ethereum.CallMsg{To: &addr, Data: callData}, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to call contract: %w", err)
	}

	var reserves struct {
		Reserve0           *big.Int
		Reserve1           *big.Int
		BlockTimestampLast uint32
	}
	if err := parsedABI.UnpackIntoInterface(&reserves, "getReserves", result); err != nil {
		return 0, 0, fmt.Errorf("failed to unpack result: %w", err)
	}

	if reserves.Reserve0.Sign() == 0 || reserves.Reserve1.Sign() == 0 {
		return 0, 0, fmt.Errorf("one of the reserves is zero")
	}

	// reserves expressed in whole tokens
	units0 := new(big.Float).Quo(new(big.Float).SetInt(reserves.Reserve0), pow10(pool.Token0Decimals))
	units1 := new(big.Float).Quo(new(big.Float).SetInt(reserves.Reserve1), pow10(pool.Token1Decimals))

	// token0 is the base and token1 the quote, unless the pair is the other way round
	base, quote := units0, units1
	if pool.Invert {
		base, quote = units1, units0
	}

	priceFloat, _ := new(big.Float).Quo(quote, base).Float64()
	quoteFloat, _ := quote.Float64()
	return priceFloat, quoteFloat, nil
}

func pow10(exp int64) *big.Float {
	return new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(exp), nil))
}

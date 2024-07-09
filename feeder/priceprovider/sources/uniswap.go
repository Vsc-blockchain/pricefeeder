package sources

import (
	"context"
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
	Uniswap = "uniswap"
)

var _ types.FetchPricesFunc = UniswapPriceUpdate

const (
	publicNodeURL      = "https://ethereum-rpc.publicnode.com"
	uniswapPairABIJSON = `[{"constant":true,"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"_reserve0","type":"uint112"},{"internalType":"uint112","name":"_reserve1","type":"uint112"},{"internalType":"uint32","name":"_blockTimestampLast","type":"uint32"}],"payable":false,"stateMutability":"view","type":"function"}]`
	ethUsdtPairAddress = "0x0d4a11d5EEaaC28EC3F61d100daF4d40471f1852" // ETH/USDT Uniswap V2 pair contract address
	ethUsdcPairAddress = "0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc" // ETH/USDC Uniswap V2 pair contract address
	vsgEthPairAddress  = "0x844a5ccdc91e604f55085adfc02e4d52c8227099" // VSG/ETH pair contract address
)

// UniswapPriceUpdate returns the prices for given symbols or an error.
func UniswapPriceUpdate(symbols set.Set[types.Symbol], logger zerolog.Logger) (rawPrices map[types.Symbol]float64, err error) {
	client, err := ethclient.Dial(publicNodeURL)
	if err != nil {
		logger.Err(err).Msg("failed to connect to the Ethereum client")
		metrics.PriceSourceCounter.WithLabelValues(Uniswap, "false").Inc()
		return nil, err
	}

	parsedABI, err := abi.JSON(strings.NewReader(uniswapPairABIJSON))
	if err != nil {
		logger.Err(err).Msg("failed to parse contract ABI")
		metrics.PriceSourceCounter.WithLabelValues(Uniswap, "false").Inc()
		return nil, err
	}

	rawPrices = make(map[types.Symbol]float64)

	// Get price for ETH/USDT pair
	ethPriceInUSDT, err := getPrice(client, parsedABI, ethUsdtPairAddress, 18, 6, logger)
	if err != nil {
		logger.Err(err).Msg("failed to fetch price for ETH/USDT")
	} else {
		logger.Debug().Msg(fmt.Sprintf("fetched price for ETH/USDT: %f", ethPriceInUSDT))
	}

	// Get price for ETH/USDC pair
	ethPriceInUSDC, err := getPrice(client, parsedABI, ethUsdcPairAddress, 18, 6, logger)
	if err != nil {
		logger.Err(err).Msg("failed to fetch price for ETH/USDC")
	} else {
		logger.Debug().Msg(fmt.Sprintf("fetched price for ETH/USDC: %f", ethPriceInUSDC))
	}

	// Calculate average ETH price in USD
	ethPriceInUSD := (ethPriceInUSDT + ethPriceInUSDC) / 2
	rawPrices["ETHUSD"] = ethPriceInUSD

	// Get price for VSG/ETH pair
	vsgPriceInETH, err := getPrice(client, parsedABI, vsgEthPairAddress, 18, 18, logger)
	if err != nil {
		logger.Err(err).Msg("failed to fetch price for VSG/ETH")
	} else {
		logger.Debug().Msg(fmt.Sprintf("fetched price for VSG/ETH: %f", vsgPriceInETH))
	}

	// Calculate VSG price in USD
	vsgPriceInUSD := vsgPriceInETH * ethPriceInUSD
	rawPrices["VSGUSD"] = vsgPriceInUSD

	metrics.PriceSourceCounter.WithLabelValues(Uniswap, "true").Inc()
	return rawPrices, nil
}

func getPrice(client *ethclient.Client, parsedABI abi.ABI, pairAddress string, token0Decimals, token1Decimals int64, logger zerolog.Logger) (float64, error) {
	addr := common.HexToAddress(pairAddress)
	callData, err := parsedABI.Pack("getReserves")
	if err != nil {
		return 0, fmt.Errorf("failed to pack call data: %v", err)
	}

	msg := ethereum.CallMsg{
		To:   &addr,
		Data: callData,
	}

	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call contract: %v", err)
	}

	var reserves struct {
		Reserve0           *big.Int
		Reserve1           *big.Int
		BlockTimestampLast uint32
	}

	err = parsedABI.UnpackIntoInterface(&reserves, "getReserves", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack result: %v", err)
	}

	reserve0 := new(big.Float).SetInt(reserves.Reserve0)
	reserve1 := new(big.Float).SetInt(reserves.Reserve1)

	// Adjust for token decimals
	decimals0 := new(big.Float).SetInt(big.NewInt(0).Exp(big.NewInt(10), big.NewInt(token0Decimals), nil))
	decimals1 := new(big.Float).SetInt(big.NewInt(0).Exp(big.NewInt(10), big.NewInt(token1Decimals), nil))

	price := new(big.Float).Quo(reserve1, reserve0)
	price.Quo(price, decimals1)
	price.Mul(price, decimals0)

	priceFloat, _ := price.Float64()

	return priceFloat, nil
}

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
	vsgEthPairAddress  = "0x1E9348B71EcBaaa14EFF7B4B6186B78d1A9B9B70" // VSG/ETH pair contract address
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
	ethPriceInUSDT, err := getPrice(client, parsedABI, ethUsdtPairAddress, 18, 6, false, logger)
	if err != nil {
		logger.Err(err).Msg("failed to fetch price for ETH/USDT")
		return nil, err
	} else {
		logger.Debug().Msg(fmt.Sprintf("fetched price for ETH/USDT: %f", ethPriceInUSDT))
	}

	// Get price for ETH/USDC pair
	ethPriceInUSDC, err := getPrice(client, parsedABI, ethUsdcPairAddress, 18, 6, true, logger)
	if err != nil {
		logger.Err(err).Msg("failed to fetch price for ETH/USDC")
		return nil, err
	} else {
		logger.Debug().Msg(fmt.Sprintf("fetched price for ETH/USDC: %f", ethPriceInUSDC))
	}

	// Calculate average ETH price in USD
	// if price diverts too much, return an error
	if ethPriceInUSDT/ethPriceInUSDC > 1.25 || ethPriceInUSDT/ethPriceInUSDC < 0.8 {
		logger.Err(fmt.Errorf("price deviation too high: %f/%f", ethPriceInUSDT, ethPriceInUSDC)).Msg("price deviation too high")
		return nil, fmt.Errorf("price deviation too high: %f/%f", ethPriceInUSDT, ethPriceInUSDC)
	}

	ethPriceInUSD := (ethPriceInUSDT + ethPriceInUSDC) / 2
	rawPrices["ETHUSD"] = ethPriceInUSD

	// Get price for VSG/ETH pair
	vsgPriceInETH, err := getPrice(client, parsedABI, vsgEthPairAddress, 18, 18, false, logger)
	if err != nil {
		logger.Err(err).Msg("failed to fetch price for VSG/ETH")
		return nil, err
	} else {
		logger.Debug().Msg(fmt.Sprintf("fetched price for VSG/ETH: %f", vsgPriceInETH))
	}

	// Calculate VSG price in USD
	vsgPriceInUSD := vsgPriceInETH * ethPriceInUSD
	rawPrices["VSGUSD"] = vsgPriceInUSD

	metrics.PriceSourceCounter.WithLabelValues(Uniswap, "true").Inc()
	return rawPrices, nil
}

func getPrice(client *ethclient.Client, parsedABI abi.ABI, pairAddress string, token0Decimals, token1Decimals int64, reverse bool, logger zerolog.Logger) (float64, error) {
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
	logger.Debug().Msg(fmt.Sprintf("fetched reserves: %s, %s", reserve0.String(), reserve1.String()))

	if reserve0.Cmp(big.NewFloat(0)) == 0 || reserve1.Cmp(big.NewFloat(0)) == 0 {
		return 0, fmt.Errorf("one of the reserves is zero")
	}

	// Adjust for token decimals
	decimals0 := new(big.Float).SetInt(big.NewInt(0).Exp(big.NewInt(10), big.NewInt(token0Decimals), nil))
	decimals1 := new(big.Float).SetInt(big.NewInt(0).Exp(big.NewInt(10), big.NewInt(token1Decimals), nil))

	var price *big.Float

	if reverse {
		price = new(big.Float).Quo(reserve0, reserve1)
		price.Quo(price, decimals1)
		price.Mul(price, decimals0)
	} else {
		price = new(big.Float).Quo(reserve1, reserve0)
		price.Quo(price, decimals1)
		price.Mul(price, decimals0)
	}

	priceFloat, _ := price.Float64()

	return priceFloat, nil
}

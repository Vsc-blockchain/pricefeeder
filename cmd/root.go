package cmd

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/vsc-blockchain/pricefeeder/config"
	"github.com/vsc-blockchain/pricefeeder/feeder"
	"github.com/vsc-blockchain/pricefeeder/feeder/eventstream"
	"github.com/vsc-blockchain/pricefeeder/feeder/priceposter"
	"github.com/vsc-blockchain/pricefeeder/feeder/priceprovider"
	"github.com/vsc-blockchain/pricefeeder/utils"
)

func setupLogger() zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()
	// Default level is INFO, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return zerolog.New(os.Stderr).With().Timestamp().Logger()
}

// handleInterrupt listens for SIGINT and gracefully shuts down the feeder.
func handleInterrupt(logger zerolog.Logger, f *feeder.Feeder) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		<-interrupt
		logger.Info().Msg("shutting down gracefully")
		f.Close()
		os.Exit(1)
	}()
}

var rootCmd = &cobra.Command{
	Use:   "pricefeeder",
	Short: "Pricefeeder daemon for posting prices to VSC Chain",
	Run: func(cmd *cobra.Command, args []string) {
		logger := setupLogger()

		utils.InitSDKConfig()

		c := config.MustGet()

		eventStream := eventstream.Dial(c.WebsocketEndpoint, c.GRPCEndpoint, c.EnableTLS, logger)
		priceProvider := priceprovider.NewAggregatePriceProvider(c.ExchangesToPairToSymbolMap, c.DataSourceConfigMap, logger)
		kb, valAddr, feederAddr := config.GetAuth(c.FeederMnemonic)

		if c.ValidatorAddr != nil {
			valAddr = *c.ValidatorAddr
		}
		pricePoster := priceposter.Dial(c.GRPCEndpoint, c.ChainID, c.EnableTLS, kb, valAddr, feederAddr, logger)

		f := feeder.NewFeeder(eventStream, priceProvider, pricePoster, logger)
		f.Run()
		defer f.Close()

		handleInterrupt(logger, f)

		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":8080", nil)

		select {}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

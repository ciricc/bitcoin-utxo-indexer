package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/scanner"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/di"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxostoreservice"
	"github.com/rs/zerolog"
	"github.com/samber/do"
)

func main() {
	i := do.New()

	do.Provide(i, di.NewConfig)
	do.Provide(i, di.NewLogger)
	do.Provide(i, di.NewShutdowner)
	do.Provide(i, di.NewRedisClient)
	do.Provide(i, di.NewScannerState)
	do.Provide(i, di.NewBitcoinBlocksIterator)
	do.Provide(i, di.NewBlockchainScanner)
	do.Provide(i, di.NewUTXOStore)
	do.Provide(i, di.NewUTXOLevelDB)
	do.Provide(i, di.NewUTXOLevelDBStore)
	do.Provide(i, di.NewLevelDBTxManager)
	do.Provide(i, di.NewUTXOStoreService)

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scanner, err := do.Invoke[*scanner.Scanner[*blockchain.Block]](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create scanner")
	}

	utxStoreService, err := do.Invoke[*utxostoreservice.UTXOStoreService](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create utxo store service")
	}

	go func() {
		if err := scanner.Start(ctx, func(ctx context.Context, block *blockchain.Block) error {
			logger.Info().Str("hash", block.GetHash().String()).Msg("got new block")

			if err := utxStoreService.AddFromBlock(ctx, block); err != nil {
				return fmt.Errorf("failed to store UTXO from block: %w", err)
			}

			logger.Info().Str("hash", block.GetHash().String()).Msg("scanned new block")

			return nil
		}); err != nil {
			logger.Fatal().Err(err).Msg("failed to start scanner")
		}
	}()

	go func() {
		<-ctx.Done()

		if err := i.Shutdown(); err != nil {
			logger.Fatal().Err(err).Msg("failed to shutdown the system")
		}

		os.Exit(0)
	}()

	if err := i.ShutdownOnSIGTERM(); err != nil {
		logger.Fatal().Err(err).Msg("failed to shutdown the service")
	}
}

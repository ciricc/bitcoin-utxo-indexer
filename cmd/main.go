package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/ciricc/btc-utxo-indexer/config"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/scanner"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/state"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/di"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	utxoservice "github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/service"
	"github.com/rs/zerolog"
	"github.com/samber/do"
	"google.golang.org/grpc"
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
	do.Provide(i, di.NewGRPCServer)
	do.Provide(i, di.NewUTXOGRPCHandlers)

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to invoke configuration")
	}

	scanner, err := do.Invoke[*scanner.Scanner[*blockchain.Block]](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create scanner")
	}

	utxStoreService, err := do.Invoke[*utxoservice.Service](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create utxo store service")
	}

	scannerState, err := do.Invoke[*state.KeyValueScannerState](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to invoke scanner's state")
	}

	grpcServer, err := do.Invoke[*grpc.Server](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to invoke grpc server")
	}

	go func() {
		<-ctx.Done()
		grpcServer.Stop()
	}()

	go func() {
		ln, err := net.Listen("tcp", cfg.UTXO.Service.GRPC.Address)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to listen grpc server")
		}

		if err := grpcServer.Serve(ln); err != nil {
			logger.Fatal().Err(err).Msg("failed to start grpc server")
		}
	}()

	go func() {
		if err := scanner.Start(ctx, func(ctx context.Context, block *blockchain.Block) error {
			logger.Info().Str("hash", block.GetHash().String()).Msg("got new block")

			if err := utxStoreService.AddFromBlock(ctx, block); err != nil {
				return fmt.Errorf("failed to store UTXO from block: %w", err)
			}

			// TODO: This must be in the same transaction or AddFromBlock must be written with the
			// last proceeded block hash identifier to prevent the "not found output" error
			if err := scannerState.UpdateLastScannedBlockHash(ctx, block.Hash.String()); err != nil {
				return fmt.Errorf("failed to update last scanner block hash: %w", err)
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

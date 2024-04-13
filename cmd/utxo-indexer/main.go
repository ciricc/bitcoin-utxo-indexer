package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ciricc/btc-utxo-indexer/config"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/app"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/scanner"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/state"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/chainstatemigration"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/di"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/migrationmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/restclient"
	utxoservice "github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/service"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/samber/do"
	"github.com/uptrace/uptrace-go/uptrace"
	"google.golang.org/grpc"
)

func main() {
	mainContainer := do.New()
	app.ProvideCommonDeps(mainContainer)

	logger, err := do.Invoke[*zerolog.Logger](mainContainer)
	if err != nil {
		panic(err)
	}

	if err := migrateFromChainstate(); err != nil {
		logger.Fatal().Err(err).Msg("failed to migrate from chainstate")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	utxoContainer := do.New()

	app.ProvideCommonDeps(utxoContainer)
	app.ProvideRedisDeps(utxoContainer)
	app.ProvideBitcoinCoreDeps(utxoContainer)
	app.ProvideUTXOStoreDeps(utxoContainer)
	app.ProvideUTXOServiceDeps(utxoContainer)

	if err := initUptrace(utxoContainer); err != nil {
		logger.Fatal().Err(err).Msg("failed to init uptrace tracing")
	}

	if err := runUTXOGRPCServer(ctx, mainContainer, utxoContainer); err != nil {
		logger.Fatal().Err(err).Msg("failed to run UTXO grpc server")
	}

	scannerContainer := do.New()

	app.ProvideCommonDeps(scannerContainer)
	app.ProvideRedisDeps(scannerContainer)
	app.ProvideBitcoinCoreDeps(scannerContainer)
	app.ProvideScannerDeps(scannerContainer)
	app.ProvideUTXOStoreDeps(scannerContainer)

	if err := runUTXOScanner(ctx, mainContainer, utxoContainer, scannerContainer); err != nil {
		logger.Fatal().Err(err).Msg("failed to run UTXO scanner")
	}

	go func() {
		<-ctx.Done()

		if err := scannerContainer.Shutdown(); err != nil {
			logger.Fatal().Err(err).Msg("failed to shutdown the scanner container")
		}

		if err := mainContainer.Shutdown(); err != nil {
			logger.Fatal().Err(err).Msg("failed to shutdown the main container")
		}

		if err := utxoContainer.Shutdown(); err != nil {
			logger.Fatal().Err(err).Msg("failed to shutdown the utxo service container")
		}

		os.Exit(0)
	}()

	if err := mainContainer.ShutdownOnSIGTERM(); err != nil {
		logger.Fatal().Err(err).Msg("failed to shutdown the service")
	}
}

func migrateFromChainstate() error {
	chainstateContainer := do.New()

	app.ProvideCommonDeps(chainstateContainer)
	app.ProvideUTXOStoreDeps(chainstateContainer)
	app.ProvideRedisDeps(chainstateContainer)
	app.ProvideChainstateDeps(chainstateContainer)
	app.ProvideBitcoinCoreDeps(chainstateContainer)

	if err := runChainstateMigration(chainstateContainer); err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	if err := deleteOldVersionsData(); err != nil {
		return fmt.Errorf("failed to delete old versions data: %w", err)
	}

	return nil
}

func runUTXOScanner(
	ctx context.Context,
	mainContainer *do.Injector,
	utxoContainer *do.Injector,
	scannerContainer *do.Injector,
) error {
	logger, err := do.Invoke[*zerolog.Logger](mainContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke logger: %w", err)
	}

	cfg, err := do.Invoke[*config.Config](mainContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke configuration: %w", err)
	}

	utxoStoreService, err := do.Invoke[*utxoservice.Service[redis.Pipeliner, *utxostore.Store[redis.Pipeliner]]](utxoContainer)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create utxo store service")
	}

	scanner, err := do.Invoke[*scanner.Scanner[*blockchain.Block]](scannerContainer)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create scanner")
	}

	scannerState, err := do.Invoke[*state.InMemoryState](scannerContainer)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to invoke scanner's state")
	}

	if cfg.Scanner.Enabled {
		logger.Info().Msg("scanner started")

		go func() {
			if err := scanner.Start(ctx, func(ctx context.Context, block *blockchain.Block) error {
				logger.Info().Str("hash", block.GetHash().String()).Msg("got new block")

				isAlreadyScanned := false
				if err := utxoStoreService.AddFromBlock(ctx, block); err != nil {
					if !errors.Is(err, utxoservice.ErrBlockAlreadyStored) {
						logger.Err(err).Str("hash", block.GetHash().String()).Msg("failed to store UTXO from block")

						return fmt.Errorf("failed to store UTXO from block: %w", err)
					} else {
						isAlreadyScanned = true
					}
				}

				logger.Info().Str("hash", block.GetHash().String()).Msg("got new block")
				if err := scannerState.UpdateLastScannedBlockHash(ctx, block.Hash.String()); err != nil {
					return fmt.Errorf("failed to update last scanner block hash: %w", err)
				}

				if !isAlreadyScanned {
					logger.Info().Str("hash", block.GetHash().String()).Msg("scanned new block")
				} else {
					logger.Info().Str("hash", block.GetHash().String()).Msg("block already scanned")
				}

				return nil
			}); err != nil {
				logger.Fatal().Err(err).Msg("failed to start scanner")
			}
		}()
	} else {
		logger.Warn().Msg("scanner is disabled")
	}

	return nil
}

func initUptrace(utxoContainer *do.Injector) error {
	cfg, err := do.Invoke[*config.Config](utxoContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke the configuration: %w", err)
	}

	logger, err := do.Invoke[*zerolog.Logger](utxoContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke the logger: %w", err)
	}

	redisClient, err := do.Invoke[*redis.Client](utxoContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke redis client: %w", err)
	}

	if cfg.Uptrace.DSN == "" {
		logger.Warn().Msg("uptrace DSN not configured, tracing disabled")
		return nil
	}

	if err := redisotel.InstrumentTracing(redisClient); err != nil {
		return fmt.Errorf("failed to init uptrace for redis client: %w", err)
	}

	if err := redisotel.InstrumentMetrics(redisClient); err != nil {
		return fmt.Errorf("failed to init uptrace for redis client: %w", err)
	}

	uptrace.ConfigureOpentelemetry(
		uptrace.WithDSN(cfg.Uptrace.DSN),
		uptrace.WithServiceName(cfg.Name),
		uptrace.WithServiceVersion(cfg.Version),
		uptrace.WithDeploymentEnvironment(cfg.Environment),
	)

	go func() {

		uptrace.ForceFlush(context.Background())
		time.Sleep(time.Second)
	}()

	return nil
}

func runUTXOGRPCServer(ctx context.Context, mainContainer *do.Injector, utxoContainer *do.Injector) error {
	logger, err := do.Invoke[*zerolog.Logger](mainContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke logger: %w", err)
	}

	cfg, err := do.Invoke[*config.Config](mainContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke configuration: %w", err)
	}

	grpcServer, err := do.Invoke[*grpc.Server](utxoContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke grpc server: %w", err)
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

		logger.Info().Str("address", cfg.UTXO.Service.GRPC.Address).Msg("UTXO grpc server started")

		if err := grpcServer.Serve(ln); err != nil {
			logger.Fatal().Err(err).Msg("failed to start grpc server")
		}
	}()

	return nil
}

func deleteOldVersionsData() error {
	i := do.New()

	app.ProvideCommonDeps(i)
	app.ProvideUTXOStoreDeps(i)
	app.ProvideRedisDeps(i)

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		return fmt.Errorf("failed to invoke logger: %w", err)
	}

	logger.Info().Msg("deleting old versions data")

	migrationManager, err := do.Invoke[*migrationmanager.Manager](i)
	if err != nil {
		return fmt.Errorf("failed to invoke migration manager: %w", err)
	}

	for {
		currentVersions, err := migrationManager.GetVersions(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get versions: %w", err)
		}

		if len(currentVersions) <= 1 {
			logger.Debug().Msg("no more versions to delete")

			break
		}

		do.Override(i, di.GetUTXOStoreConstructor[redis.Pipeliner]())

		utxoStore, err := do.Invoke[*utxostore.Store[redis.Pipeliner]](i)
		if err != nil {
			return fmt.Errorf("failed to invoke UTXO store: %w", err)
		}

		err = utxoStore.Flush(context.Background())
		if err != nil {
			return fmt.Errorf("failed to flush UTXO store: %w", err)
		}

		logger.Info().Int64("version", currentVersions[0]).Msg("deleting version")
		err = migrationManager.DeleteVersion(context.Background(), currentVersions[0])
		if err != nil {
			return fmt.Errorf("failed to delete version: %w", err)
		}

	}

	return nil
}

func runChainstateMigration(chainstateContainer *do.Injector) error {
	cfg, err := do.Invoke[*config.Config](chainstateContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke configuration: %w", err)
	}

	oldUtxoStore, err := do.Invoke[*utxostore.Store[redis.Pipeliner]](chainstateContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke UTXO store: %w", err)
	}

	migrationManager, err := do.Invoke[*migrationmanager.Manager](chainstateContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke migration manager: %w", err)
	}

	do.Override(chainstateContainer, di.GetUTXOStoreConstructor[redis.Pipeliner]())

	if err := migrationManager.SetMigrationID(1); err != nil {
		return fmt.Errorf("failed to set migration id: %w", err)
	}

	chainstateDB, err := do.Invoke[*chainstate.DB](chainstateContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke chainstate db: %w", err)
	}

	logger, err := do.Invoke[*zerolog.Logger](chainstateContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke logger: %w", err)
	}

	restClient, err := do.Invoke[*restclient.RESTClient](chainstateContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke rest client: %w", err)
	}

	chainstateBlockHash, err := chainstateDB.GetBlockHash(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get chainstate block hash: %w", err)
	}

	blockInfo, err := restClient.GetBlockHeader(context.Background(), chainstateBlockHash)
	if err != nil {
		return fmt.Errorf("failed to get block info: %w", err)
	}

	currentHeightFromUTXOStore, err := oldUtxoStore.GetBlockHeight(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get current utxo store block height: %w", err)
	}

	if blockInfo.GetHeight() <= currentHeightFromUTXOStore {
		logger.Info().Msg("the chainstate has older version of the utxo")

		return nil
	}

	utxoStore, err := do.Invoke[*utxostore.Store[redis.Pipeliner]](chainstateContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke UTXO store: %w", err)
	}

	txManager, err := do.Invoke[*txmanager.TransactionManager[redis.Pipeliner]](chainstateContainer)
	if err != nil {
		return fmt.Errorf("failed to invoke tx manager: %w", err)
	}

	migration := chainstatemigration.NewMigrator(
		logger,
		chainstateDB,
		utxoStore,
		txManager,
		cfg.ChainstateMigration.BatchSize,
		blockInfo.Height,
	)

	err = migration.Migrate(context.Background())
	if err != nil {
		return fmt.Errorf("migrate error: %w", err)
	}

	_, err = migrationManager.UpdateVersion(context.Background())
	if err != nil {
		return fmt.Errorf("update database version error: %w", err)
	}

	return nil
}

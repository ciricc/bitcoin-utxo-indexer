package di

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/config"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/chainstatemigration"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/restclient"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
	"github.com/samber/do"
)

func GetMigratorConstructor[T any]() do.Provider[*chainstatemigration.Migrator] {
	return func(i *do.Injector) (*chainstatemigration.Migrator, error) {
		cfg, err := do.Invoke[*config.Config](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke configuration: %w", err)
		}

		chainstateDB, err := do.Invoke[*chainstate.DB](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke chainstate db: %w", err)
		}

		logger, err := do.Invoke[*zerolog.Logger](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke logger: %w", err)
		}

		restClient, err := do.Invoke[*restclient.RESTClient](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke rest client: %w", err)
		}

		chainstateBlockHash, err := chainstateDB.GetBlockHash(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get chainstate block hash: %w", err)
		}

		blockInfo, err := restClient.GetBlockHeader(context.Background(), chainstateBlockHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get block info: %w", err)
		}

		utxoStore, err := do.Invoke[*utxostore.Store[T]](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke UTXO store: %w", err)
		}

		migration := chainstatemigration.NewMigrator(
			logger,
			chainstateDB,
			utxoStore,
			cfg.ChainstateMigration.BatchSize,
			blockInfo.Height,
		)

		return migration, nil
	}
}

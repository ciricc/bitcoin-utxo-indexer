package di

import (
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/config"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/counters"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/migrationmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do"
	"github.com/syndtr/goleveldb/leveldb"
)

func NewChainstateLevelDB(i *do.Injector) (*leveldb.DB, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke config: %w", err)
	}

	db, err := leveldb.OpenFile(cfg.BlockchainState.Path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB: %w", err)
	}

	return db, nil
}

func NewRedisCounters(i *do.Injector) (*counters.RedisCounters, error) {
	redisCli, err := do.Invoke[*redis.Client](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke redis client: %w", err)
	}

	counters, err := counters.NewRedisCounters(redisCli)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis counters: %w", err)
	}

	return counters, nil
}

func NewUTXOStoreMigrationManager(i *do.Injector) (*migrationmanager.Manager, error) {
	redisSets, err := do.Invoke[sets.SetsWithTxManager[redis.Pipeliner]](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke redis: %w", err)
	}

	manager, err := migrationmanager.NewManager("utxo_store", redisSets)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke migration manager: %w", err)
	}

	return manager, nil
}

func NewChainstateDB(i *do.Injector) (*chainstate.DB, error) {
	chainstateLevelDB, err := do.Invoke[*leveldb.DB](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke leveldb: %w", err)
	}

	db, err := chainstate.NewDB(chainstateLevelDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create chainstate db: %w", err)
	}

	return db, nil

}

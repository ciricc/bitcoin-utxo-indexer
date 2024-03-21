package di

import (
	"fmt"
	"net/url"
	"os"

	"github.com/IBM/sarama"
	"github.com/ciricc/btc-utxo-indexer/config"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoinblocksiterator"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/scanner"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/state"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	leveldbkvstore "github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/providers/leveldb"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/logger"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/shutdown"
	leveldbtx "github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/drivers/leveldb"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	utxoservice "github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/service"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/philippgille/gokv/encoding"
	redis "github.com/philippgille/gokv/redis"
	"github.com/rs/zerolog"
	"github.com/samber/do"
	"github.com/syndtr/goleveldb/leveldb"
	configLoader "gitlab.enigmagroup.tech/enigma/evo-wallet/evo-backend/common/config"
)

func NewBlockchainScanner(i *do.Injector) (*scanner.Scanner[*blockchain.Block], error) {
	bitcoinBlocksIterator, err := do.Invoke[*bitcoinblocksiterator.BitcoinBlocksIterator](i)
	if err != nil {
		return nil, fmt.Errorf("invoke bitcin blocs iterator error: %w", err)
	}

	state, err := do.Invoke[*state.KVStoreState](i)
	if err != nil {
		return nil, fmt.Errorf("invoke state error: %w", err)
	}

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("invoke logger error: %w", err)
	}

	scanner, err := scanner.NewScannerWithState(
		bitcoinBlocksIterator,
		state,
		scanner.WithLogger(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create scanner: %w", err)
	}

	return scanner, nil
}

func NewBitcoinBlocksIterator(i *do.Injector) (*bitcoinblocksiterator.BitcoinBlocksIterator, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("invoke config error: %w", err)
	}

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("invoke logger error: %w", err)
	}

	nodeRestURL, err := url.Parse(cfg.BlockchainNode.RestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse blockchain node rest url: %w", err)
	}

	bitcoinBlocksIterator, err := bitcoinblocksiterator.NewBitcoinBlocksIterator(
		nodeRestURL,
		bitcoinblocksiterator.WithBlockHeadersBufferSize(cfg.BlockchainBlocksIterator.BlockHeadersBufferSize),
		bitcoinblocksiterator.WithConcurrentBlocksDownloadLimit(cfg.BlockchainBlocksIterator.ConcurrentBlocksDownloadLimit),
		bitcoinblocksiterator.WithLogger(logger),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create bitcoin blocks iterator: %w", err)
	}

	return bitcoinBlocksIterator, nil
}

func NewUTXOLevelDB(i *do.Injector) (*leveldb.DB, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke configuration: %w", err)
	}

	db, err := leveldb.OpenFile(cfg.UTXO.Storage.LevelDB.Path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb file: %w", err)
	}

	return db, nil
}

func NewUTXOLevelDBStore(i *do.Injector) (keyvaluestore.StoreWithTxManager[*leveldb.Transaction], error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke configuration: %w", err)
	}

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke logger: %w", err)
	}

	levelDB, err := do.Invoke[*leveldb.DB](i)
	if err != nil {
		return nil, fmt.Errorf("failed to ijnvoke leveldb store: %w", err)
	}

	logger.Info().Str("filePath", cfg.UTXO.Storage.LevelDB.Path).Msg("initalizing leveldb UTXO")

	store, err := leveldbkvstore.NewLevelDBStore(levelDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create leveldb store: %w", err)
	}

	return store, nil
}

func NewRedisClient(i *do.Injector) (*redis.Client, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("invoke config error: %w", err)
	}

	client, err := redis.NewClient(redis.Options{
		Codec:   encoding.JSON,
		Address: cfg.ScannerState.RedisStore.Host,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	return &client, nil
}

func NewScannerState(i *do.Injector) (*state.KVStoreState, error) {
	redisStore, err := do.Invoke[*redis.Client](i)
	if err != nil {
		return nil, fmt.Errorf("invoke key value store error: %w", err)
	}

	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("invoke config error: %w", err)
	}

	state := state.NewStateWithKeyValueStore(
		cfg.ScannerState.StartFromBlockHash,
		redisStore,
	)

	return state, nil
}

func NewShutdowner(i *do.Injector) (*shutdown.Shutdowner, error) {
	kafkaSyncProducer, err := do.Invoke[sarama.SyncProducer](i)
	if err != nil {
		return nil, fmt.Errorf("invoke kafka sync producer error: %w", err)
	}

	redisClient, err := do.Invoke[*redis.Client](i)
	if err != nil {
		return nil, fmt.Errorf("invoke redis client error: %w", err)
	}

	levelDB, err := do.Invoke[*leveldb.DB](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke leveldb store: %w", err)
	}

	shutdowner := shutdown.NewShutdowner(
		shutdown.NewShutdownFromCloseable(kafkaSyncProducer),
		shutdown.NewShutdownFromCloseable(redisClient),
		shutdown.NewShutdownFromCloseable(levelDB),
	)

	return shutdowner, nil
}

func NewUTXOStore(i *do.Injector) (*utxostore.Store, error) {
	levelDBStore, err := do.Invoke[keyvaluestore.StoreWithTxManager[*leveldb.Transaction]](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke redis client: %w", err)
	}

	store, err := utxostore.New(levelDBStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create UTXO store: %w", err)
	}

	return store, nil
}

func NewUTXOStoreService(i *do.Injector) (*utxoservice.Service, error) {
	utxoStore, err := do.Invoke[*utxostore.Store](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke UTXO store: %w", err)
	}

	utxoKVStore, err := do.Invoke[keyvaluestore.StoreWithTxManager[*leveldb.Transaction]](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke UTXO KV store: %w", err)
	}

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke logger: %w", err)
	}

	txManager, err := do.Invoke[*txmanager.TransactionManager[*leveldb.Transaction]](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke tx manager: %w", err)
	}

	utxoStoreService := utxoservice.New(
		utxoStore,
		txManager,
		utxoKVStore,
		&utxoservice.ServiceOptions{
			Logger: logger,
		},
	)

	return utxoStoreService, nil
}

func NewLogger(i *do.Injector) (*zerolog.Logger, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("invoke config error: %w", err)
	}

	log := logger.NewLogger(cfg)

	return &log, nil
}

func NewLevelDBTxManager(i *do.Injector) (*txmanager.TransactionManager[*leveldb.Transaction], error) {
	levelDB, err := do.Invoke[*leveldb.DB](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke leveldb store: %w", err)
	}

	txManager := txmanager.New(leveldbtx.NewLevelDBTransactionFactory(levelDB))

	return txManager, nil
}

func NewConfig(_ *do.Injector) (*config.Config, error) {
	configFilePath := "config/config.yml"

	configFilePathFromEnv := os.Getenv("CONFIG_FILE")
	if configFilePathFromEnv != "" {
		configFilePath = configFilePathFromEnv
	}

	var cfg config.Config
	if err := configLoader.LoadServiceConfig(&cfg, configFilePath); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	return &cfg, nil
}

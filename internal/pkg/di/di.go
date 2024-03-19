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
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/logger"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/shutdown"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/unspentoutputs/store"
	"github.com/philippgille/gokv/encoding"
	"github.com/philippgille/gokv/leveldb"
	redis "github.com/philippgille/gokv/redis"
	"github.com/rs/zerolog"
	"github.com/samber/do"
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

func NewLevelDBStore(i *do.Injector) (*keyvaluestore.LevelDBStore, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke configuration: %w", err)
	}

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke logger: %w", err)
	}

	logger.Info().Str("filePath", cfg.UTXO.Storage.LevelDB.Path).Msg("initalizing leveldb UTXO")

	store, err := keyvaluestore.NewLevelDBStore(&keyvaluestore.StoreOptions{
		Path: cfg.UTXO.Storage.LevelDB.Path,
	})
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

	levelDBStore, err := do.Invoke[*leveldb.Store](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke leveldb store: %w", err)
	}

	shutdowner := shutdown.NewShutdowner(
		shutdown.NewShutdownFromCloseable(kafkaSyncProducer),
		shutdown.NewShutdownFromCloseable(redisClient),
		shutdown.NewShutdownFromCloseable(levelDBStore),
	)

	return shutdowner, nil
}

func NewUTXOStore(i *do.Injector) (*store.UnspentOutputsStore, error) {
	levelDBStore, err := do.Invoke[*keyvaluestore.LevelDBStore](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke redis client: %w", err)
	}
	store, err := store.New(levelDBStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create UTXO store: %w", err)
	}

	return store, nil
}

func NewLogger(i *do.Injector) (*zerolog.Logger, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("invoke config error: %w", err)
	}

	log := logger.NewLogger(cfg)

	return &log, nil
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

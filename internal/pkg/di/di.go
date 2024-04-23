package di

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/ciricc/btc-utxo-indexer/config"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoinblocksiterator"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoinconfig"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/scanner"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/state"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/providers/inmemorykvstore"
	leveldbkvstore "github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/providers/inmemorykvstore/leveldb"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/providers/rediskvstore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/logger"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/migrationmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/providers/redissets"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/shutdown"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/drivers/inmemorytx"
	leveldbtx "github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/drivers/leveldb"
	redistx "github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/drivers/redis"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/restclient"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/checkpointstore"
	utxoservice "github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/service"
	grpchandlers "github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/transport/grpc"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxospending"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/ciricc/btc-utxo-indexer/pkg/api/grpc/TxOuts_V1"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/samber/do"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewBlockchainScanner(i *do.Injector) (*scanner.Scanner[*blockchain.Block], error) {
	bitcoinBlocksIterator, err := do.Invoke[*bitcoinblocksiterator.BitcoinBlocksIterator](i)
	if err != nil {
		return nil, fmt.Errorf("invoke bitcin blocs iterator error: %w", err)
	}

	state, err := do.Invoke[*state.InMemoryState](i)
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

	nodeRESTClient, err := do.Invoke[*restclient.RESTClient](i)
	if err != nil {
		return nil, fmt.Errorf("invoke universal bitcoin rest client error: %w", err)
	}

	bitcoinBlocksIterator, err := bitcoinblocksiterator.NewBitcoinBlocksIterator(
		nodeRESTClient,
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

	db, err := leveldb.OpenFile(cfg.UTXO.Storage.LevelDB.Path, &opt.Options{
		WriteBuffer: 512 * opt.MiB,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb file: %w", err)
	}

	return db, nil
}

func NewRedisKeyValueStore(i *do.Injector) (keyvaluestore.StoreWithTxManager[redis.Pipeliner], error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke configuration: %w", err)
	}

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke logger: %w", err)
	}

	redis, err := do.Invoke[*redis.Client](i)
	if err != nil {
		return nil, fmt.Errorf("invoke redis error: %w", err)
	}

	logger.Info().Str("redisHost", cfg.UTXO.Storage.Redis.Host).Msg("initalizing redis UTXO")

	return rediskvstore.New(redis), nil
}

func NewInMemoryStore(i *do.Injector) (*inmemorykvstore.Store, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("invoke config error: %w", err)
	}

	store, err := inmemorykvstore.New(
		inmemorykvstore.WithPersistencePath(cfg.UTXO.Storage.InMemory.PersistenceFilePath),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create leveldb store: %w", err)
	}

	return store, nil
}

func NewKeyValueInMemoryStore(i *do.Injector) (keyvaluestore.StoreWithTxManager[*inmemorykvstore.Store], error) {
	store, err := do.Invoke[*inmemorykvstore.Store](i)
	if err != nil {
		return nil, fmt.Errorf("failed to create leveldb store: %w", err)
	}

	return store, nil
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

	client := redis.NewClient(&redis.Options{
		Addr:           cfg.UTXO.Storage.Redis.Host,
		Username:       cfg.UTXO.Storage.Redis.Username,
		Password:       cfg.UTXO.Storage.Redis.Password,
		DB:             cfg.UTXO.Storage.Redis.DB,
		ReadTimeout:    cfg.UTXO.Storage.Redis.ConnectionPool.ReadTimeout,
		WriteTimeout:   cfg.UTXO.Storage.Redis.ConnectionPool.WriteTimeout,
		MaxIdleConns:   cfg.UTXO.Storage.Redis.ConnectionPool.MaxIdleConns,
		MaxActiveConns: cfg.UTXO.Storage.Redis.ConnectionPool.MaxActiveConns,
	})

	return client, nil
}

func GetScannerStateWithInMemoryStoreByUTXOStoreType[T any]() do.Provider[*state.InMemoryState] {
	return func(i *do.Injector) (*state.InMemoryState, error) {
		logger, err := do.Invoke[*zerolog.Logger](i)
		if err != nil {
			return nil, fmt.Errorf("invoke logger error: %w", err)
		}

		utxoStore, err := do.Invoke[*utxostore.Store[T]](i)
		if err != nil {
			return nil, fmt.Errorf("invoke UTXO store error: %w", err)
		}

		lastUTXOBlockHash, err := utxoStore.GetBlockHash(context.Background())
		if err != nil && !errors.Is(err, utxostore.ErrBlockHashNotFound) {
			return nil, fmt.Errorf("failed to get last UTXO block hash: %w", err)
		}

		restClient, err := do.Invoke[*restclient.RESTClient](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke rest client: %w", err)
		}

		genesisBlockHash, err := restClient.GetBlockHash(context.Background(), 0)
		if err != nil {
			return nil, fmt.Errorf("faield to get genesis block hash: %w", err)
		}

		if lastUTXOBlockHash == "" {
			lastUTXOBlockHash = genesisBlockHash.String()
		}

		logger.Info().Str("lastUTXOBlockHash", lastUTXOBlockHash).Msg("got last UTXO block hash")

		state := state.NewInMemoryState(
			lastUTXOBlockHash,
		)

		return state, nil
	}
}

func GetScannerStateConstructor[T any]() do.Provider[*state.KeyValueScannerState] {
	return func(i *do.Injector) (*state.KeyValueScannerState, error) {
		kvStore, err := do.Invoke[keyvaluestore.StoreWithTxManager[T]](i)
		if err != nil {
			return nil, fmt.Errorf("invoke key value store error: %w", err)
		}

		restClient, err := do.Invoke[*restclient.RESTClient](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke rest client: %w", err)
		}

		genesisBlockHash, err := restClient.GetBlockHash(context.Background(), 0)
		if err != nil {
			return nil, fmt.Errorf("faield to get genesis block hash: %w", err)
		}

		state := state.NewStateWithKeyValueStore(
			genesisBlockHash.String(),
			kvStore,
		)

		return state, nil
	}
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

	// levelDB, err := do.Invoke[*leveldb.DB](i)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to invoke leveldb store: %w", err)
	// }

	// inMemory, err := do.Invoke[*inmemorykvstore.Store](i)
	// if err != nil {
	// 	return nil, fmt.Errorf("invoke in-memory store error: %w", err)
	// }

	shutdowner := shutdown.NewShutdowner(
		shutdown.NewShutdownFromCloseable(kafkaSyncProducer),
		shutdown.NewShutdownFromCloseable(redisClient),
		// shutdown.NewShutdownFromCloseable(levelDB),
		// shutdown.NewShutdownFromCloseable(inMemory),
	)

	return shutdowner, nil
}

func GetUTXOStoreConstructor[T any]() do.Provider[*utxostore.Store[T]] {
	return func(i *do.Injector) (*utxostore.Store[T], error) {
		kvStore, err := do.Invoke[keyvaluestore.StoreWithTxManager[T]](i)
		if err != nil {
			return nil, fmt.Errorf("invoke redis store error: %w", err)
		}

		txManager, err := do.Invoke[*txmanager.TransactionManager[T]](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke transaction manager: %w", err)
		}

		sets, err := do.Invoke[sets.SetsWithTxManager[T]](i)
		if err != nil {
			return nil, fmt.Errorf("invoke sets error: %w", err)
		}

		migrationManager, err := do.Invoke[*migrationmanager.Manager](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke migration manager: %w", err)
		}

		databaseVersion, err := migrationManager.GetVersion(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get database version: %w", err)
		}

		checkpointsStore, err := do.Invoke[*checkpointstore.CheckpointStore](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke checkpoints store: %w", err)
		}

		store, err := utxostore.New(strconv.FormatInt(databaseVersion, 10), kvStore, sets, txManager, checkpointsStore)
		if err != nil {
			return nil, fmt.Errorf("failed to create UTXO store: %w", err)
		}

		return store, nil
	}
}

func NewRedisSets(i *do.Injector) (sets.SetsWithTxManager[redis.Pipeliner], error) {
	redis, err := do.Invoke[*redis.Client](i)
	if err != nil {
		return nil, fmt.Errorf("invoke redis error: %w", err)
	}

	return redissets.New(redis), nil
}

func GetUTXOServiceConstructor[T redis.Pipeliner]() do.Provider[*utxoservice.UTXOService] {
	return func(i *do.Injector) (*utxoservice.UTXOService, error) {
		utxoStore, err := do.Invoke[*utxostore.Store[T]](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke UTXO store: %w", err)
		}

		logger, err := do.Invoke[*zerolog.Logger](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke logger: %w", err)
		}

		bitcoinConfig, err := do.Invoke[*bitcoinconfig.BitcoinConfig](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke bitcoin config: %w", err)
		}

		utxoSpender, err := do.Invoke[*utxospending.UTXOSpender](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke UTXO spender: %w", err)
		}

		utxoStoreService := utxoservice.NewRedisUTXOService(
			utxoStore,
			bitcoinConfig,
			&utxoservice.ServiceOptions{
				Logger: logger,
			},
			utxoSpender,
		)

		return utxoStoreService, nil
	}
}

func GetUTXOSpenderConstructor[T any]() do.Provider[*utxospending.UTXOSpender] {
	return func(i *do.Injector) (*utxospending.UTXOSpender, error) {
		utxoStore, err := do.Invoke[*utxostore.Store[T]](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke UTXO store: %w", err)
		}

		bitcoinConfig, err := do.Invoke[*bitcoinconfig.BitcoinConfig](i)
		if err != nil {
			return nil, fmt.Errorf("failed to invoke bitcoin config: %w", err)
		}

		spender, err := utxospending.NewUTXOSpender(
			utxoStore,
			bitcoinConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create UTXO spender: %w", err)
		}

		return spender, nil
	}
}

func NewCheckpointsStore(i *do.Injector) (*checkpointstore.CheckpointStore, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke config: %w", err)
	}

	return checkpointstore.NewCheckpointStore(cfg.UTXO.Storage.CheckpointFilePath), nil
}

func NewBitcoinConfig(i *do.Injector) (*bitcoinconfig.BitcoinConfig, error) {
	restClient, err := do.Invoke[*restclient.RESTClient](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke rest client: %w", err)
	}

	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke configuration: %w", err)
	}

	config, err := bitcoinconfig.New(restClient, cfg.BlockchainParams.Decimals)
	if err != nil {
		return nil, fmt.Errorf("failed to create bitcoin config: %w", err)
	}

	return config, nil
}

func NewUniversalBitcoinRESTClient(i *do.Injector) (*restclient.RESTClient, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke configuration: %w", err)
	}

	nodeURL, err := url.Parse(cfg.BlockchainNode.RestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse blockchain node rest url: %w", err)
	}

	restClient, err := restclient.New(nodeURL, &restclient.RESTClientOptions{
		RequestTimeout: 5 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create rest client: %w", err)
	}

	return restClient, nil
}

func NewSlogLogger(i *do.Injector) (*slog.Logger, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("invoke config error: %w", err)
	}

	log := logger.NewSlogLogger(cfg)

	return log, nil
}

func NewLogger(i *do.Injector) (*zerolog.Logger, error) {
	cfg, err := do.Invoke[*config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("invoke config error: %w", err)
	}

	log := logger.NewLogger(cfg)

	return &log, nil
}

func NewBlockchainV1GRPCHandlers(i *do.Injector) (*grpchandlers.TxOutsV1BlockchainHandlers, error) {
	service, err := do.Invoke[*utxoservice.UTXOService](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invo UTXO service: %w", err)
	}

	return grpchandlers.NewV1BlockchainHandlers(service), nil
}

func NewAddressV1GRPCHandlers(i *do.Injector) (*grpchandlers.TxOutsV1AddressesHandlers, error) {
	service, err := do.Invoke[*utxoservice.UTXOService](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invo UTXO service: %w", err)
	}

	return grpchandlers.NewV1AddressHandlers(service), nil
}

func NewTxOutsGatewayServeMux(i *do.Injector) (*runtime.ServeMux, error) {
	addressHandlers, err := do.Invoke[*grpchandlers.TxOutsV1AddressesHandlers](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke address grpc handlers: %w", err)
	}

	blockchainHandlers, err := do.Invoke[*grpchandlers.TxOutsV1BlockchainHandlers](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke blockchain grpc handlers: %w", err)
	}

	ctx := context.Background()

	mux := runtime.NewServeMux()

	err = TxOuts_V1.RegisterAddressesHandlerServer(ctx, mux, addressHandlers)
	if err != nil {
		return nil, fmt.Errorf("failed to register addresses handlers: %w", err)
	}

	err = TxOuts_V1.RegisterBlockchainHandlerServer(ctx, mux, blockchainHandlers)
	if err != nil {
		return nil, fmt.Errorf("failed to register blockchain handlers: %w", err)
	}

	return mux, nil
}

func NewGRPCServer(i *do.Injector) (*grpc.Server, error) {
	addressHandlers, err := do.Invoke[*grpchandlers.TxOutsV1AddressesHandlers](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke address grpc handlers: %w", err)
	}

	blockchainHandlers, err := do.Invoke[*grpchandlers.TxOutsV1BlockchainHandlers](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke blockchain grpc handlers: %w", err)
	}

	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	TxOuts_V1.RegisterAddressesServer(server, addressHandlers)
	TxOuts_V1.RegisterBlockchainServer(server, blockchainHandlers)

	reflection.Register(server)

	return server, nil
}

func NewRedisTxManager(i *do.Injector) (*txmanager.TransactionManager[redis.Pipeliner], error) {
	redis, err := do.Invoke[*redis.Client](i)
	if err != nil {
		return nil, fmt.Errorf("redis invoke error: %w", err)
	}

	txManager := txmanager.New(redistx.NewRedisTransactionFactory(redis))

	return txManager, nil
}

func NewInMemoryTxManager(i *do.Injector) (*txmanager.TransactionManager[*inmemorykvstore.Store], error) {
	inMemoryStore, err := do.Invoke[*inmemorykvstore.Store](i)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke leveldb store: %w", err)
	}

	txManager := txmanager.New(inmemorytx.NewInMemoryTransactionFactory(inMemoryStore))

	return txManager, nil
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
	if err := config.LoadServiceConfig(&cfg, configFilePath); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	return &cfg, nil
}

package app

import (
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/di"
	"github.com/redis/go-redis/v9"
	"github.com/samber/do"
)

func ProvideRedisDeps(i *do.Injector) {
	do.Provide(i, di.NewRedisSets)
	do.Provide(i, di.NewRedisCounters)
	do.Provide(i, di.NewRedisTxManager)
}

func ProvideUTXOStoreDeps(i *do.Injector) {
	do.Provide(i, di.NewUTXOStoreMigrationManager)
	do.Provide(i, di.GetUTXOStoreConstructor[redis.Pipeliner]())
	do.Provide(i, di.NewRedisKeyValueStore)
	do.Provide(i, di.NewRedisClient)
	do.Provide(i, di.NewCheckpointsStore)
	do.Provide(i, di.GetUTXOSpenderConstructor[redis.Pipeliner]())
}

func ProvideBitcoinCoreDeps(i *do.Injector) {
	do.Provide(i, di.NewBitcoinConfig)
	do.Provide(i, di.NewUniversalBitcoinRESTClient)
	do.Provide(i, di.NewBitcoinBlocksIterator)
}

func ProvideMigratorDeps(i *do.Injector) {
	do.Provide(i, di.GetMigratorConstructor[redis.Pipeliner]())
}

func ProvideChainstateDeps(i *do.Injector) {
	do.Provide(i, di.NewChainstateLevelDB)
	do.Provide(i, di.NewChainstateDB)
}

func ProvideCommonDeps(i *do.Injector) {
	do.Provide(i, di.NewConfig)
	do.Provide(i, di.NewLogger)
	do.Provide(i, di.NewSlogLogger)
}

func ProvideUTXOServiceDeps(i *do.Injector) {
	do.Provide(i, di.GetUTXOServiceConstructor[redis.Pipeliner]())
	do.Provide(i, di.NewAddressV1GRPCHandlers)
	do.Provide(i, di.NewBlockchainV1GRPCHandlers)
	do.Provide(i, di.NewGRPCServer)
	do.Provide(i, di.NewTxOutsGatewayServeMux)
}

func ProvideScannerDeps(i *do.Injector) {
	do.Provide(i, di.GetScannerStateWithInMemoryStoreByUTXOStoreType[redis.Pipeliner]())
	do.Provide(i, di.NewBlockchainScanner)
}

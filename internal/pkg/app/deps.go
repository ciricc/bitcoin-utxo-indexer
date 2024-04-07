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
}

func ProvideBitcoinCoreDeps(i *do.Injector) {
	do.Provide(i, di.NewBitcoinConfig)
	do.Provide(i, di.NewUniversalBitcoinRESTClient)
	do.Provide(i, di.NewBitcoinBlocksIterator)
}

func ProvideChainstateDeps(i *do.Injector) {
	do.Provide(i, di.NewChainstateLevelDB)
	do.Provide(i, di.NewChainstateDB)
}

func ProvideCommonDeps(i *do.Injector) {
	do.Provide(i, di.NewConfig)
	do.Provide(i, di.NewLogger)
}

func ProvideUTXOServiceDeps(i *do.Injector) {
	do.Provide(i, di.GetUTXOServiceConstructor[redis.Pipeliner]())
	do.Provide(i, di.GeUTXOGRPCHandlersConstructor[redis.Pipeliner]())
	do.Provide(i, di.NewGRPCServer)
}

func ProvideScannerDeps(i *do.Injector) {
	do.Provide(i, di.NewScannerStateWithInMemoryStore)

	do.Provide(i, di.NewBlockchainScanner)
}

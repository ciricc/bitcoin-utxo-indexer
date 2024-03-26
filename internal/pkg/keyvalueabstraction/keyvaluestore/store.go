package keyvaluestore

import "github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"

type Store interface {
	// Get retrieves the value for the given key.
	Get(key string, v any) (found bool, err error)

	// Set sets the key to the given value.
	Set(key string, v any) error

	// Delete deletes the key from the store.
	Delete(key string) error

	// ListKeys iterates over all keys in the store and calls the given function for each key.
	ListKeys(match string, si func(key string, getValue func(v interface{}) error) (stop bool, err error)) error
}

type StoreWithTxManager[T any] interface {
	Store

	// WithTx returns a new KeyValueStoreWithTxManager instance with the given transaction.
	WithTx(tx txmanager.Transaction[T]) (StoreWithTxManager[T], error)
}

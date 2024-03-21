package keyvaluestore

import "github.com/ciricc/btc-utxo-indexer/internal/pkg/txmanager"

type KeyValueStore[T any] interface {
	// Get retrieves the value for the given key.
	Get(key string, v any) (found bool, err error)

	// Set sets the key to the given value.
	Set(key string, v any) error

	// Delete deletes the key from the store.
	Delete(key string) error

	// ListKeys iterates over all keys in the store and calls the given function for each key.
	ListKeys(si func(key string, getValue func(v interface{}) error) (ok bool, err error)) error

	// WithTx returns a new KeyValueStore instance with the given transaction.
	WithTx(tx txmanager.Transaction[T]) (KeyValueStore[T], error)
}

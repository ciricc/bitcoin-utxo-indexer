package keyvaluestore

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
)

type Store interface {
	// Get retrieves the value for the given key.
	Get(ctx context.Context, key string, v any) (found bool, err error)

	// MulGet retrievels the values for the given keys
	MulGet(ctx context.Context, allocValue func(ctx context.Context, key string) any, keys ...string) error

	// Set sets the key to the given value.
	Set(ctx context.Context, key string, v any) error

	// Delete deletes the key from the store.
	Delete(ctx context.Context, key string) error

	// ListKeys iterates over all keys in the store and calls the given function for each key.
	ListKeys(ctx context.Context, match string, si func(key string, getValue func(v interface{}) error) (stop bool, err error)) error

	// Flush deletes all keys
	Flush(ctx context.Context) error

	// DeleteByPattern deletes all keys matching pattern
	DeleteByPattern(ctx context.Context, pattern string) error
}

type StoreWithTxManager[T any] interface {
	Store

	// WithTx returns a new KeyValueStoreWithTxManager instance with the given transaction.
	WithTx(tx txmanager.Transaction[T]) (StoreWithTxManager[T], error)
}

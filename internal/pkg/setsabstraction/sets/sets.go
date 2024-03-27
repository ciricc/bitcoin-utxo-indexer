package sets

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
)

type Sets interface {
	GetSet(ctx context.Context, key string) ([]string, error)
	AddToSet(ctx context.Context, key string, values ...string) error
	RemoveFromSet(ctx context.Context, key string, values ...string) error
	DeleteSet(ctx context.Context, key string) error
}

type SetsWithTxManager[T any] interface {
	Sets

	// WithTx returns a new SetsWithTxManager instance with the given transaction.
	WithTx(tx txmanager.Transaction[T]) (SetsWithTxManager[T], error)
}

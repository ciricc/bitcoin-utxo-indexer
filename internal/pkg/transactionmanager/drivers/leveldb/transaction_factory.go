package leveldbtx

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/syndtr/goleveldb/leveldb"
)

type LevelDBTransactionOpener interface {
	// OpenTransaction opens the leveldb transaction
	OpenTransaction() (*leveldb.Transaction, error)
}

// NewLevelDBTransactionFactory returns a factory for the leveldb transactions
// It requires an instance of the leveldb transaction opener
func NewLevelDBTransactionFactory(db LevelDBTransactionOpener) txmanager.TransactionFactory[*leveldb.Transaction] {
	return func(ctx context.Context, _ txmanager.Settings) (context.Context, txmanager.Transaction[*leveldb.Transaction], error) {
		tx, err := db.OpenTransaction()
		if err != nil {
			return ctx, nil, fmt.Errorf("failed to open transaction: %w", err)
		}

		return ctx, &LevelDBTransaction{
			tx: tx,
		}, nil
	}
}

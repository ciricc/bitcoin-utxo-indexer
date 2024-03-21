package trmfactory

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/txmanager"
	"github.com/syndtr/goleveldb/leveldb"
)

type LevelDBTransactionAdapter struct {
	tx *leveldb.Transaction
}

func (l *LevelDBTransactionAdapter) Commit(_ context.Context) error {
	if l.tx == nil {
		return ErrAlreadyCommitted
	}

	defer l.close()

	return l.tx.Commit()
}

func (l *LevelDBTransactionAdapter) close() {
	if l.tx == nil {
		return
	}

	l.tx = nil
}

func (l *LevelDBTransactionAdapter) Transaction() *leveldb.Transaction {
	return l.tx
}

func (l *LevelDBTransactionAdapter) Rollback(_ context.Context) error {
	l.tx.Discard()
	defer l.close()

	return nil
}

var _ txmanager.Transaction[*leveldb.Transaction] = (*LevelDBTransactionAdapter)(nil)

func NewLevelDBTrmFactory(db *leveldb.DB) txmanager.TxFactory[*leveldb.Transaction] {
	return func(ctx context.Context) (context.Context, txmanager.Transaction[*leveldb.Transaction], error) {
		tx, err := db.OpenTransaction()
		if err != nil {
			return ctx, nil, fmt.Errorf("failed to open transaction: %w", err)
		}

		return ctx, &LevelDBTransactionAdapter{
			tx: tx,
		}, nil
	}
}

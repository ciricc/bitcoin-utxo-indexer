package leveldbtx

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/syndtr/goleveldb/leveldb"
)

type LevelDBTransaction struct {
	tx *leveldb.Transaction
}

func (l *LevelDBTransaction) Commit(_ context.Context) error {
	if l.tx == nil {
		return ErrAlreadyCommitted
	}

	defer l.close()

	return l.tx.Commit()
}

func (l *LevelDBTransaction) close() {
	if l.tx == nil {
		return
	}

	l.tx = nil
}

func (l *LevelDBTransaction) Transaction() *leveldb.Transaction {
	return l.tx
}

func (l *LevelDBTransaction) Rollback(_ context.Context) error {
	l.tx.Discard()
	defer l.close()

	return nil
}

var _ txmanager.Transaction[*leveldb.Transaction] = (*LevelDBTransaction)(nil)

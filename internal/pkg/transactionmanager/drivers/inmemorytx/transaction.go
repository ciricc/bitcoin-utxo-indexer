package inmemorytx

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/providers/inmemorykvstore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
)

type InMemoryTransaction struct {
	tx *inmemorykvstore.Store
}

func NewInMemoryTransaction(tx *inmemorykvstore.Store) (*InMemoryTransaction, error) {
	return &InMemoryTransaction{
		tx: tx,
	}, nil
}

// Commit implements txmanager.Transaction.
func (i *InMemoryTransaction) Commit(ctx context.Context) error {
	return nil
}

// Rollback implements txmanager.Transaction.
func (i *InMemoryTransaction) Rollback(ctx context.Context) error {
	return nil
}

// Transaction implements txmanager.Transaction.
func (i *InMemoryTransaction) Transaction() *inmemorykvstore.Store {
	return i.tx
}

var _ txmanager.Transaction[*inmemorykvstore.Store] = (*InMemoryTransaction)(nil)

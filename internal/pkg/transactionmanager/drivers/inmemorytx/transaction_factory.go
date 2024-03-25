package inmemorytx

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/providers/inmemorykvstore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
)

func NewInMemoryTransactionFactory(store *inmemorykvstore.Store) txmanager.TransactionFactory[*inmemorykvstore.Store] {
	return func(ctx context.Context) (context.Context, txmanager.Transaction[*inmemorykvstore.Store], error) {
		tx, err := NewInMemoryTransaction(store)
		if err != nil {
			return ctx, nil, fmt.Errorf("create tx error: %w", err)
		}

		return ctx, tx, nil
	}
}

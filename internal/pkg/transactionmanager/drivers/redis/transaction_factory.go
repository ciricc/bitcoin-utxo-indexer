package redistx

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/redis/go-redis/v9"
)

func NewRedisTransactionFactory(client *redis.Client) txmanager.TransactionFactory[*redis.Tx] {
	return func(ctx context.Context) (context.Context, txmanager.Transaction[*redis.Tx], error) {
		tx, err := NewTransaction(ctx, client)
		if err != nil {
			return ctx, nil, fmt.Errorf("failed to open transaction: %w", err)
		}

		return ctx, tx, nil
	}
}

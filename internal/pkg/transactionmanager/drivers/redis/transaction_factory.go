package redistx

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/redis/go-redis/v9"
)

func NewRedisTransactionFactory(client *redis.Client) txmanager.TransactionFactory[redis.Pipeliner] {
	return func(ctx context.Context, settings txmanager.Settings) (context.Context, txmanager.Transaction[redis.Pipeliner], error) {
		var defaultSettings *Settings

		if s, ok := settings.(*Settings); ok {
			defaultSettings = s
		} else {
			defaultSettings = NewSettings()
		}

		tx, err := NewRedisTransaction(ctx, client, defaultSettings)
		if err != nil {
			return ctx, nil, fmt.Errorf("failed to open transaction: %w", err)
		}

		return ctx, tx, nil
	}
}

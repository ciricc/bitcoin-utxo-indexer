package redistx

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisTransaction wraps a Redis transaction (TxPipeliner) and implements the Transaction interface.
type RedisTransaction struct {
	client *redis.Client
	tx     redis.Pipeliner
}

// NewRedisTransaction creates and returns a new RedisTransaction.
func NewRedisTransaction(ctx context.Context, client *redis.Client, settings *Settings) (*RedisTransaction, error) {
	// Initiate a new transaction.
	tx := client.TxPipeline()
	rtx := &RedisTransaction{client: client, tx: tx}

	return rtx, nil
}

// Commit attempts to commit the transaction.
func (r *RedisTransaction) Commit(ctx context.Context) error {
	_, err := r.tx.Exec(ctx)
	return err
}

// Rollback aborts the transaction. Redis does not support rollback in the traditional sense,
// so this is a no-op but included for interface compatibility.
func (r *RedisTransaction) Rollback(ctx context.Context) error {
	// Redis transactions automatically rollback on error. This function is for compatibility.
	r.tx.Discard()

	return nil
}

// Transaction returns the underlying Redis transaction (Pipeliner).
func (r *RedisTransaction) Transaction() redis.Pipeliner {
	return r.tx
}

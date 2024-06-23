package redissets

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/redis/go-redis/v9"
)

type RedisSets struct {
	redis redis.Cmdable
}

func New(redis redis.Cmdable) *RedisSets {
	return &RedisSets{
		redis: redis,
	}
}

// AddToSet implements sets.SetsWithTxManager.
func (r *RedisSets) AddToSet(ctx context.Context, key string, values ...string) error {
	err := r.redis.SAdd(ctx, key, convertStringsToInterfaces(values)...).Err()
	if err != nil {
		return fmt.Errorf("SADD error: %w", err)
	}

	return nil
}

// DeleteSet implements sets.SetsWithTxManager.
func (r *RedisSets) DeleteSet(ctx context.Context, key string) error {
	err := r.redis.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("delete error: %w", err)
	}

	return nil
}

// GetSet implements sets.SetsWithTxManager.
func (r *RedisSets) GetSet(ctx context.Context, key string) ([]string, error) {
	values, err := r.redis.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("SMEMBERS error: %w", err)
	}

	return values, nil
}

// RemoveFromSet implements sets.SetsWithTxManager.
func (r *RedisSets) RemoveFromSet(ctx context.Context, key string, values ...string) error {
	err := r.redis.SRem(ctx, key, convertStringsToInterfaces(values)...).Err()
	if err != nil {
		return fmt.Errorf("SREM error: %w", err)
	}

	return nil
}

// WithTx implements sets.SetsWithTxManager.
func (r *RedisSets) WithTx(tx txmanager.Transaction[redis.Pipeliner]) (sets.SetsWithTxManager[redis.Pipeliner], error) {
	return &RedisSets{
		redis: tx.Transaction(),
	}, nil
}

func convertStringsToInterfaces(values []string) []interface{} {
	interfaces := make([]interface{}, len(values))
	for i, v := range values {
		interfaces[i] = v
	}
	return interfaces
}

var _ sets.SetsWithTxManager[redis.Pipeliner] = (*RedisSets)(nil)

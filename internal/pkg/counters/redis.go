package counters

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/redis/go-redis/v9"
)

type RedisCounters struct {
	r redis.Cmdable
}

func NewRedisCounters(redisClient redis.Cmdable) (*RedisCounters, error) {
	return &RedisCounters{
		r: redisClient,
	}, nil
}

func (r *RedisCounters) WithTx(tx txmanager.Transaction[redis.Pipeliner]) *RedisCounters {
	return &RedisCounters{
		r: tx.Transaction(),
	}
}

func (r *RedisCounters) GetCounter(ctx context.Context, name string) (int64, error) {
	res, err := r.r.Get(ctx, name).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrNotFound
		}

		return 0, fmt.Errorf("failed to get counter: %w", err)
	}

	counterVal, err := strconv.ParseInt(res, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse counter: %w", err)
	}

	return counterVal, nil
}

func (r *RedisCounters) IncrCounter(ctx context.Context, name string) (int64, error) {
	newVal, err := r.r.Incr(ctx, name).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrNotFound
		}

		return 0, fmt.Errorf("failed to increment counter: %w", err)
	}

	return newVal, nil
}

func (r *RedisCounters) IncrCounterBy(ctx context.Context, name string, value int64) (int64, error) {
	newVal, err := r.r.IncrBy(ctx, name, value).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrNotFound
		}

		return 0, fmt.Errorf("failed to increment counter by value: %w", err)
	}

	return newVal, nil
}

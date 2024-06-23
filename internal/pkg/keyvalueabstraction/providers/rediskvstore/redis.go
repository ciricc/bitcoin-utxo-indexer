package rediskvstore

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/encoding"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"

	// "github.com/philippgille/gokv/encoding"
	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	redis  redis.Cmdable
	encode encoding.Codec
}

func New(redis redis.Cmdable) *RedisStore {
	return &RedisStore{
		redis:  redis,
		encode: encoding.MsgPack,
	}
}

// Delete implements keyvaluestore.StoreWithTxManager.
func (r *RedisStore) Delete(ctx context.Context, key string) error {
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete error: %w", err)
	}

	return nil
}

// Get implements keyvaluestore.StoreWithTxManager.
func (r *RedisStore) Get(ctx context.Context, key string, v any) (found bool, err error) {
	res, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}

		return false, fmt.Errorf("get element error: %w", err)
	}

	err = r.encode.Unmarshal([]byte(res), v)
	if err != nil {
		return true, fmt.Errorf("unmarshal error: %w", err)
	}

	return true, nil
}

// ListKeys implements keyvaluestore.StoreWithTxManager.
func (r *RedisStore) ListKeys(ctx context.Context, match string, si func(key string, getValue func(v interface{}) error) (ok bool, err error)) error {
	if match == "" {
		match = "*"
	}

	iter := r.redis.Scan(ctx, 0, match, 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		if si != nil {
			stop, err := si(key, func(v interface{}) error {
				res, err := r.redis.Get(ctx, key).Result()
				if err != nil {
					return fmt.Errorf("get iterator value error: %w", err)
				}

				err = r.encode.Unmarshal([]byte(res), v)
				if err != nil {
					return fmt.Errorf("unmarshal iterator value error: %w", err)
				}

				return nil
			})
			if err != nil {
				return err
			}

			if stop {
				break
			}
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("iterate error: %w", err)
	}

	return nil
}

// Set implements keyvaluestore.StoreWithTxManager.
func (r *RedisStore) Set(ctx context.Context, key string, v any) error {
	val, err := r.encode.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode error: %w", err)
	}

	err = r.redis.Set(ctx, key, string(val), redis.KeepTTL).Err()
	if err != nil {
		return fmt.Errorf("set error: %w", err)
	}

	return nil
}

// WithTx implements keyvaluestore.StoreWithTxManager.
func (r *RedisStore) WithTx(tx txmanager.Transaction[redis.Pipeliner]) (keyvaluestore.StoreWithTxManager[redis.Pipeliner], error) {
	return &RedisStore{
		redis:  tx.Transaction(),
		encode: r.encode,
	}, nil
}

func (r *RedisStore) DeleteByPattern(ctx context.Context, pattern string) error {
	return r.ListKeys(ctx, pattern, func(key string, getValue func(v interface{}) error) (ok bool, err error) {
		return false, r.Delete(ctx, key)
	})
}

func (r *RedisStore) Flush(ctx context.Context) error {
	if err := r.redis.FlushAll(ctx).Err(); err != nil {
		return fmt.Errorf("fialed to flush: %w", err)
	}

	return nil
}

func (r *RedisStore) MulGet(ctx context.Context, allocValue func(ctx context.Context, key string) any, keys ...string) error {
	if allocValue == nil {
		return fmt.Errorf("alloca value function is nil")
	}

	values := r.redis.MGet(ctx, keys...).Val()
	for i, val := range values {
		key := keys[i]

		valPtr := allocValue(ctx, key)

		if val == nil {
			continue
		}

		switch t := val.(type) {
		case string:
			if err := r.encode.Unmarshal([]byte(t), valPtr); err != nil {
				return fmt.Errorf("unmarshal value error: %w", err)
			}
		default:
			return fmt.Errorf("unknown value type: %v (%v): %v", t, reflect.TypeOf(t), val)
		}

	}

	return nil
}

var _ keyvaluestore.StoreWithTxManager[redis.Pipeliner] = (*RedisStore)(nil)

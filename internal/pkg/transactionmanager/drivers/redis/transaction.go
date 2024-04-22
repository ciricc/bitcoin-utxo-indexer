package redistx

import (
	"context"
	"errors"
	"sync"

	"github.com/avito-tech/go-transaction-manager/trm/drivers"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/redis/go-redis/v9"
)

// RedisTransaction wraps a Redis transaction (TxPipeliner) and implements the Transaction interface.
type RedisTransaction struct {
	client *redis.Client
	tx     redis.Pipeliner

	closed  *drivers.IsClosed
	closure *drivers.IsClosed
}

// NewRedisTransaction creates and returns a new RedisTransaction.
func NewRedisTransaction(ctx context.Context, client *redis.Client, settings *Settings) (*RedisTransaction, error) {
	// Initiate a new transaction.

	t := &RedisTransaction{
		tx:      nil,
		closure: drivers.NewIsClosed(),
		closed:  drivers.NewIsClosed(),
		client:  client,
	}

	var err error

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		err = client.Watch(ctx, func(rawRx *redis.Tx) error {
			_, err = rawRx.TxPipelined(ctx, func(p redis.Pipeliner) error {
				t.tx = p

				wg.Done()

				<-t.closure.Closed()

				return t.closure.Err()

			})

			return err
		}, settings.WatchingKeys()...)

		if t.tx == nil {
			// we didnt run the watch callback
			// so, we need to check the error
			wg.Done()
		} else {
			t.closed.CloseWithCause(err)
		}

	}()

	wg.Wait()

	if err != nil {
		return nil, err
	}

	go t.awaitDone(ctx)

	return t, nil
}

func (t *RedisTransaction) awaitDone(ctx context.Context) {
	if ctx.Done() == nil {
		return
	}

	select {
	case <-ctx.Done():
		// Rollback will be called by context.Err()
		t.closure.Close()
	case <-t.closure.Closed():
	}
}

func (t *RedisTransaction) Commit(ctx context.Context) error {
	select {
	case <-t.closed.Closed():
		_, err := t.tx.Exec(ctx)

		return err
	default:
		t.closure.Close()

		<-t.closed.Closed()

		return t.closed.Err()
	}
}

// Rollback the trm.Transaction.
func (t *RedisTransaction) Rollback(_ context.Context) error {
	select {
	case <-t.closed.Closed():
		t.tx.Discard()
		return nil
	default:
		t.closure.CloseWithCause(txmanager.ErrRollback)

		<-t.closed.Closed()

		err := t.closure.Err()
		if errors.Is(err, txmanager.ErrRollback) {
			return nil
		}

		// unreachable code, because of go-redis doesn't process error from Close
		// https://github.com/redis/go-redis/blob/v8.11.5/tx.go#L69
		// https://github.com/redis/go-redis/blob/v8.11.5/pipeline.go#L130

		return err
	}
}

// Transaction returns the underlying Redis transaction (Pipeliner).
func (r *RedisTransaction) Transaction() redis.Pipeliner {
	return r.tx
}

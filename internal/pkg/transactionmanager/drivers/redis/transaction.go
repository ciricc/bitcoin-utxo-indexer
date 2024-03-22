package redistx

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/avito-tech/go-transaction-manager/trm/drivers"
	"github.com/redis/go-redis/v9"
)

type Transaction struct {
	redis *redis.Client
	tx    *redis.Tx

	// isClosed is the channel which store closed status of the
	// pipilened level transaction
	isClosed *drivers.IsClosed

	// closure is the channel which stores closed status of the
	// top-level transaction (wrapped)
	closure *drivers.IsClosed
}

func NewTransaction(
	ctx context.Context,
	rediClient *redis.Client,
) (*Transaction, error) {
	t := &Transaction{
		isClosed: drivers.NewIsClosed(),
		closure:  drivers.NewIsClosed(),
		tx:       nil,
		redis:    rediClient,
	}

	// If the transaction can't be opened, then
	// this error will be returned from the goroutine
	var err error

	txCreator := sync.WaitGroup{}
	txCreator.Add(1)

	go func() {
		err = rediClient.Watch(ctx, func(tx *redis.Tx) error {

			// After we created a transaction, we need to run a pipe
			_, err := t.tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
				t.tx = tx

				// stop waiting the transaction creating
				txCreator.Done()

				<-t.closure.Closed()

				return t.closure.Err()
			})

			return err
		})

		// If the transaction successfully opened, but execution ended with the error
		if t.tx != nil {
			t.isClosed.CloseWithCause(err)
		} else {
			txCreator.Done()
		}
	}()

	txCreator.Wait()

	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	go t.awaitDone(ctx)

	return t, nil
}

func (t *Transaction) awaitDone(ctx context.Context) {
	if ctx.Done() == nil {
		return
	}

	select {
	case <-ctx.Done():
		// Rollback will be called by context.Err() in the go-redis transaction
		t.closure.Close()
	case <-t.isClosed.Closed():
	}
}

func (t *Transaction) Transaction() *redis.Tx {
	return t.tx
}

// Commit closes the trm.Transaction.
func (t *Transaction) Commit(ctx context.Context) error {
	select {
	case <-t.isClosed.Closed():
		return t.isClosed.Err()
	default:
		t.closure.Close()

		<-t.isClosed.Closed()

		return t.isClosed.Err()
	}
}

// Rollback the trm.Transaction.
func (t *Transaction) Rollback(_ context.Context) error {
	select {
	case <-t.isClosed.Closed():
		err := t.isClosed.Err()
		if errors.Is(err, drivers.ErrRollbackTr) {
			return nil
		}

		return err
	default:
		t.closure.CloseWithCause(drivers.ErrRollbackTr)

		<-t.isClosed.Closed()

		err := t.isClosed.Err()
		if errors.Is(err, drivers.ErrRollbackTr) {
			return nil
		}

		return err
	}
}

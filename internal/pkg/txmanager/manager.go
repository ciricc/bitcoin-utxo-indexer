package txmanager

import (
	"context"
	"fmt"
)

type Transaction[T any] interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Transaction() T
}

type TxFactory[T any] func(ctx context.Context) (context.Context, Transaction[T], error)

type TxManager[T any] struct {
	txFactory TxFactory[T]
}

func NewTxManager[T any](factory TxFactory[T]) *TxManager[T] {
	return &TxManager[T]{
		txFactory: factory,
	}
}

func (t *TxManager[T]) Do(fn func(ctx context.Context, tx Transaction[T]) error) error {
	if t.txFactory == nil {
		return fmt.Errorf("txFactory is nil")
	}

	ctx, tx, err := t.txFactory(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	err = fn(ctx, tx)
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf("failed to rollaback transaction: %w", err)
		}

		return err
	}

	if err := tx.Commit(ctx); err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf("failed to rollaback transaction: %w", err)
		}

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

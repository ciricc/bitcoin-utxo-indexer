package txmanager

import (
	"context"
	"fmt"
)

// Transaction is an interface that represents a transaction.
type Transaction[T any] interface {
	// Commit commits the transaction. It returns an error if the transaction cannot be committed.
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction. It returns an error if the transaction cannot be rolled back.
	Rollback(ctx context.Context) error

	// Transaction returns the underlying transaction.
	Transaction() T
}

// TransactionFactory is a function that creates a new transaction.
type TransactionFactory[T any] func(ctx context.Context, settings Settings) (context.Context, Transaction[T], error)

type TransactionManager[T any] struct {
	txFactory TransactionFactory[T]
}

func New[T any](factory TransactionFactory[T]) *TransactionManager[T] {
	return &TransactionManager[T]{
		txFactory: factory,
	}
}

func (t *TransactionManager[T]) Do(settings Settings, fn func(ctx context.Context, tx Transaction[T]) error) error {
	if t.txFactory == nil {
		return fmt.Errorf("txFactory is nil")
	}

	ctx, tx, err := t.txFactory(context.Background(), settings)
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

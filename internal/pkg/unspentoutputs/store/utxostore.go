package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

type Store interface {
	Get(key string, v interface{}) (found bool, err error)
	Set(key string, v interface{}) error
	ListKeys(iterator func(key string, getValue func(v interface{}) error) (stop bool, err error)) error
}

type UnspentOutputsStore struct {
	s Store

	mu *sync.RWMutex

	totalUTXOCount int64
}

func New(storer Store) (*UnspentOutputsStore, error) {
	store := &UnspentOutputsStore{
		s:  storer,
		mu: &sync.RWMutex{},

		totalUTXOCount: 0,
	}

	if err := store.syncTotalUTXOCount(); err != nil {
		return nil, err
	}

	return store, nil
}

func (u *UnspentOutputsStore) syncTotalUTXOCount() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	var totalOutputsCount int64

	err := u.s.ListKeys(func(key string, getValue func(v interface{}) error) (stop bool, err error) {

		storageKey, err := StorageKeyFromString(key)
		if err != nil {
			return true, fmt.Errorf("failed to parse storage key: %w", err)
		}

		// Only where we have a list of outputs
		if !storageKey.TypeOf(TransactionIDKeyType) {
			return false, nil
		}

		var txOutputs []*TransactionOutput

		if err := getValue(&txOutputs); err != nil {
			return false, err
		}

		totalOutputsCount += int64(len(txOutputs))

		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to sync total UTXO count: %w", err)
	}

	u.totalUTXOCount = totalOutputsCount

	return nil
}

func (u *UnspentOutputsStore) CountOutputs(
	_ context.Context,
) (int64, error) {
	return atomic.LoadInt64(&u.totalUTXOCount), nil
}

func (u *UnspentOutputsStore) GetOutputsByTxID(
	_ context.Context,
	txID string,
) ([]*TransactionOutput, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	var outputs []*TransactionOutput

	ok, err := u.s.Get(newTransactionIDKey(txID).String(), &outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to get outputs: %w", err)
	}

	if !ok {
		return nil, ErrNotFound
	}

	return outputs, nil
}

func (u *UnspentOutputsStore) AddTransactionOutputs(
	ctx context.Context,
	txID string,
	outputs []*TransactionOutput,
) error {
	return u.addTxOutputs(txID, outputs)
}

func (u *UnspentOutputsStore) addTxOutputs(txID string, outputs []*TransactionOutput) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	var i interface{}

	txIDKey := newTransactionIDKey(txID)

	found, err := u.s.Get(txIDKey.String(), &i)
	if err != nil {
		return fmt.Errorf("failed to cre current tx id outputs: %w", err)
	}
	if found {
		return nil
	}

	err = u.s.Set(txIDKey.String(), outputs)
	if err != nil {
		return fmt.Errorf("failed to store tx outputs: %w", err)
	}

	u.totalUTXOCount += int64(len(outputs))

	return nil
}

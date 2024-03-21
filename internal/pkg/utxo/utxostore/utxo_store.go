package utxostore

import (
	"context"
	"fmt"
	"sync"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
)

type Store struct {
	s  keyvaluestore.Store
	mu *sync.RWMutex
}

func New(storer keyvaluestore.Store) (*Store, error) {
	store := &Store{
		s:  storer,
		mu: &sync.RWMutex{},
	}

	return store, nil
}

func (u *Store) WithStorer(s keyvaluestore.Store) *Store {
	return &Store{
		mu: u.mu,
		s:  s,
	}
}

func (u *Store) GetOutputsByTxID(
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

func (u *Store) SpendOutput(
	_ context.Context,
	txID string,
	idx int,
) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	var outputs []*TransactionOutput
	var txIDKey = newTransactionIDKey(txID)

	found, err := u.s.Get(txIDKey.String(), &outputs)
	if err != nil {
		return fmt.Errorf("failed to get transaction outputs: %w", err)
	}

	if !found {
		return ErrNotFound
	}

	if idx < 0 || idx >= len(outputs) {
		return ErrNotFound
	}

	outputs[idx] = nil

	allSpent := true
	for _, outputs := range outputs {
		if outputs != nil {
			allSpent = false
			break
		}
	}

	if allSpent {
		err := u.s.Delete(txIDKey.String())
		if err != nil {
			return fmt.Errorf("failed to delete transaction id key: %w", err)
		}
	} else {
		err := u.s.Set(txIDKey.String(), outputs)
		if err != nil {
			return fmt.Errorf("failed to set transaction id key: %w", err)
		}
	}

	return nil
}

func (u *Store) AddTransactionOutputs(
	ctx context.Context,
	txID string,
	outputs []*TransactionOutput,
) error {
	return u.addTxOutputs(txID, outputs)
}

func (u *Store) addTxOutputs(txID string, outputs []*TransactionOutput) error {
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

	return nil
}

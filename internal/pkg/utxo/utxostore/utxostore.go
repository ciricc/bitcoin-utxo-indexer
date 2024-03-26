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

	addressUTXOIds *addressUTXOIdx
}

func New(storer keyvaluestore.Store) (*Store, error) {
	store := &Store{
		s:              storer,
		mu:             &sync.RWMutex{},
		addressUTXOIds: newAddressUTXOIndex(storer),
	}

	return store, nil
}

func (u *Store) WithStorer(storer keyvaluestore.Store) *Store {
	return &Store{
		mu:             u.mu,
		s:              storer,
		addressUTXOIds: newAddressUTXOIndex(storer),
	}
}

func (u *Store) GetBlockHeight(_ context.Context) (int64, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	blockHeightKey := newBlockheightKey()

	var currentBlockHeight int64

	_, err := u.s.Get(blockHeightKey.String(), &currentBlockHeight)
	if err != nil {
		return 0, fmt.Errorf("failed to get current block height: %w", err)
	}

	return currentBlockHeight, nil
}

func (u *Store) SetBlockHeight(_ context.Context, blockHeight int64) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	blockHeightKey := newBlockheightKey()

	err := u.s.Set(blockHeightKey.String(), blockHeight)
	if err != nil {
		return fmt.Errorf("failed to store new block height: %w", err)
	}

	return nil
}

func (u *Store) GetUnspentOutputsByAddress(_ context.Context, address string) ([]*TransactionOutput, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	txIDsWithOutputs, err := u.addressUTXOIds.getAddressUTXOTransactionIds(address)
	if err != nil {
		return nil, err
	}

	if len(txIDsWithOutputs) == 0 {
		return nil, nil
	}

	var res []*TransactionOutput

	for _, txID := range txIDsWithOutputs {
		var outputs []*TransactionOutput

		found, err := u.s.Get(newTransactionIDKey(txID, true).String(), &outputs)
		if err != nil {
			return nil, fmt.Errorf("failed to get outputs: %w", err)
		}

		if !found {
			continue
		}

		for _, output := range outputs {
			if !isSpentOutput(output) {
				res = append(res, output)
			}
		}
	}

	return res, nil
}

func (u *Store) GetOutputsByTxID(
	_ context.Context,
	txID string,
) ([]*TransactionOutput, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	var outputs []*TransactionOutput

	ok, err := u.s.Get(newTransactionIDKey(txID, true).String(), &outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to get outputs: %w", err)
	}

	if !ok {
		return nil, ErrNotFound
	}

	return outputs, nil
}

func (u *Store) SpendOutputFromRetrievedOutputs(
	_ context.Context,
	txID string,
	outputs []*TransactionOutput,
	idx int,
) (*TransactionOutput, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.spendOutput(txID, idx, outputs)
}

func (u *Store) SpendOutput(
	_ context.Context,
	txID string,
	idx int,
) (*TransactionOutput, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	var outputs []*TransactionOutput
	var txOutputsKey = newTransactionIDKey(txID, true)

	found, err := u.s.Get(txOutputsKey.String(), &outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction outputs: %w", err)
	}

	if !found {
		return nil, ErrNotFound
	}

	return u.spendOutput(txID, idx, outputs)
}

func (u *Store) spendOutput(
	txID string,
	idx int,
	outputs []*TransactionOutput,
) (*TransactionOutput, error) {
	var txOutputsKey = newTransactionIDKey(txID, true)

	if idx < 0 || idx >= len(outputs) {
		return nil, ErrNotFound
	}

	if outputs[idx] == nil {
		return nil, ErrAlreadySpent
	}

	var spentOutput = *outputs[idx]

	outputs[idx] = nil

	txOutputAddresses := map[string]struct{}{}

	// We need this variable because the output may not have an address
	// So, we can't just appolige on txOutputAddresses size > 0
	allSpent := true
	for _, output := range outputs {
		if output != nil {
			allSpent = false
			for _, address := range output.Addresses {
				txOutputAddresses[address] = struct{}{}
			}
		}

	}

	// There is no left unpent outputs
	if allSpent {
		err := u.s.Delete(txOutputsKey.String())
		if err != nil {
			return nil, fmt.Errorf("failed to delete transaction id key: %w", err)
		}
	} else {
		err := u.s.Set(txOutputsKey.String(), outputs)
		if err != nil {
			return nil, fmt.Errorf("failed to set transaction id key: %w", err)
		}
	}

	// Remove index on this tx id from the storage
	// For each address no more exising
	for _, address := range spentOutput.Addresses {
		if _, ok := txOutputAddresses[address]; !ok {
			// remove idx for this address on unspent outputs
			if err := u.addressUTXOIds.deleteAdressUTXOTransactionIds(address); err != nil {
				return nil, err
			}
		}
	}

	return &spentOutput, nil
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

	txOutputsKey := newTransactionIDKey(txID, true)

	err := u.s.Set(txOutputsKey.String(), outputs)
	if err != nil {
		return fmt.Errorf("failed to store tx outputs: %w", err)
	}

	err = u.createAddressUTXOTxIdIndex(txID, outputs)
	if err != nil {
		return err
	}

	return nil
}

func (u *Store) createAddressUTXOTxIdIndex(txID string, outputs []*TransactionOutput) error {
	for _, output := range outputs {
		for _, address := range output.Addresses {
			err := u.addressUTXOIds.addAddressUTXOTransactionIds(address, []string{txID})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func isSpentOutput(output *TransactionOutput) bool {
	return output == nil
}

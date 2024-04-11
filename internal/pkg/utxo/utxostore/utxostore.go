package utxostore

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
)

type Store[T any] struct {
	s    keyvaluestore.StoreWithTxManager[T]
	sets sets.SetsWithTxManager[T]

	addressUTXOIds *redisAddressUTXOIdx[T]

	// dbVer storing the version of the database (prefix for all keys)
	dbVer string
}

func New[T any](
	databaseVersion string,
	storer keyvaluestore.StoreWithTxManager[T],
	setsStore sets.SetsWithTxManager[T],
) (*Store[T], error) {
	store := &Store[T]{
		s:              storer,
		addressUTXOIds: newAddressUTXOIndex(databaseVersion, setsStore),
		dbVer:          databaseVersion,
		sets:           setsStore,
	}

	return store, nil
}

func (u *Store[T]) WithTx(tx txmanager.Transaction[T]) (*Store[T], error) {
	storerWithTx, err := u.s.WithTx(tx)
	if err != nil {
		return nil, err
	}

	setsWithTx, err := u.sets.WithTx(tx)
	if err != nil {
		return nil, err
	}

	return &Store[T]{
		s:              storerWithTx,
		addressUTXOIds: newAddressUTXOIndex(u.dbVer, setsWithTx),
		dbVer:          u.dbVer,
	}, nil
}

func (u *Store[T]) Flush(ctx context.Context) error {
	if err := u.s.DeleteByPattern(ctx, u.dbVer+":*"); err != nil {
		return fmt.Errorf("flush error: %w", err)
	}

	return nil
}

func (u *Store[T]) GetBlockHeight(_ context.Context) (int64, error) {
	blockHeightKey := newBlockHeightKey(u.dbVer)

	var currentBlockHeight int64

	_, err := u.s.Get(blockHeightKey.String(), &currentBlockHeight)
	if err != nil {
		return 0, fmt.Errorf("failed to get current block height: %w", err)
	}

	return currentBlockHeight, nil
}

func (u *Store[T]) GetBlockHash(_ context.Context) (string, error) {
	blockHashKey := newBlockHashKey(u.dbVer)

	var currentBlockHash string

	found, err := u.s.Get(blockHashKey.String(), &currentBlockHash)
	if err != nil {
		return "", fmt.Errorf("failed to get current block hash: %w", err)
	}

	if !found {
		return "", ErrBlockHashNotFound
	}

	return currentBlockHash, nil
}

func (u *Store[T]) SetBlockHeight(_ context.Context, blockHeight int64) error {
	blockHeightKey := newBlockHeightKey(u.dbVer)

	err := u.s.Set(blockHeightKey.String(), blockHeight)
	if err != nil {
		return fmt.Errorf("failed to store new block height: %w", err)
	}

	return nil
}

func (u *Store[T]) SetBlockHash(_ context.Context, blockHash string) error {
	blockHashKey := newBlockHashKey(u.dbVer)

	err := u.s.Set(blockHashKey.String(), blockHash)
	if err != nil {
		return fmt.Errorf("failed to store new block hash: %w", err)
	}

	return nil
}

func (u *Store[T]) RemoveAddressTxIDs(_ context.Context, address string, txIDs []string) error {
	if err := u.addressUTXOIds.deleteAdressUTXOTransactionIds(address, txIDs); err != nil {
		return fmt.Errorf("delete address UTXO tx ids error: %w", err)
	}

	return nil
}

func (u *Store[T]) GetUnspentOutputsByAddress(_ context.Context, address string) ([]*UTXOEntry, error) {
	txIDsWithOutputs, err := u.addressUTXOIds.getAddressUTXOTransactionIds(address)
	if err != nil {
		return nil, err
	}

	if len(txIDsWithOutputs) == 0 {
		return nil, nil
	}

	var res []*UTXOEntry

	for _, txID := range txIDsWithOutputs {
		var outputs []*TransactionOutput

		found, err := u.s.Get(newTransactionIDKey(u.dbVer, txID).String(), &outputs)
		if err != nil {
			return nil, fmt.Errorf("failed to get outputs: %w", err)
		}

		if !found {
			continue
		}

		for vout, output := range outputs {
			if !isSpentOutput(output) {
				addresses, err := output.GetAddresses()
				if err != nil {
					return nil, fmt.Errorf("failed to get addresses: %w", err)
				}
				foundAddr := false

				for _, addr := range addresses {
					if address == addr {
						foundAddr = true
						break
					}
				}

				if !foundAddr {
					continue
				}

				res = append(res, &UTXOEntry{
					TxID:   txID,
					Vout:   uint32(vout),
					Output: outputs[vout],
				})
			}
		}
	}

	return res, nil
}

func (u *Store[T]) GetOutputsByTxID(
	_ context.Context,
	txID string,
) ([]*TransactionOutput, error) {
	var outputs []*TransactionOutput

	ok, err := u.s.Get(newTransactionIDKey(u.dbVer, txID).String(), &outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to get outputs: %w", err)
	}

	if !ok {
		return nil, ErrNotFound
	}

	return outputs, nil
}

func (u *Store[T]) SpendOutputFromRetrievedOutputs(
	_ context.Context,
	txID string,
	outputs []*TransactionOutput,
	idx int,
) ([]string, *TransactionOutput, error) {

	return u.spendOutput(txID, idx, outputs)
}

func (u *Store[T]) SpendAllOutputs(
	_ context.Context,
	txID string,
) error {
	var txOutputsKey = newTransactionIDKey(u.dbVer, txID)
	if err := u.s.Delete(txOutputsKey.String()); err != nil {
		return fmt.Errorf("delete outputs error: %w", err)
	}

	return nil
}

func (u *Store[T]) SpendOutput(
	_ context.Context,
	txID string,
	idx int,
) ([]string, *TransactionOutput, error) {
	var outputs []*TransactionOutput
	var txOutputsKey = newTransactionIDKey(u.dbVer, txID)

	found, err := u.s.Get(txOutputsKey.String(), &outputs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get transaction outputs: %w", err)
	}

	if !found {
		return nil, nil, ErrNotFound
	}

	return u.spendOutput(txID, idx, outputs)
}

func (u *Store[T]) spendOutput(
	txID string,
	idx int,
	outputs []*TransactionOutput,
) ([]string, *TransactionOutput, error) {
	var txOutputsKey = newTransactionIDKey(u.dbVer, txID)

	if idx < 0 || idx >= len(outputs) {
		return nil, nil, ErrNotFound
	}

	if outputs[idx] == nil {
		return nil, nil, ErrAlreadySpent
	}

	var spentOutput = *outputs[idx]

	outputs[idx] = nil

	unspentTxOutputAddresses := map[string]struct{}{}

	// We need this variable because the output may not have an address
	// So, we can't just appolige on txOutputAddresses size > 0
	allSpent := true
	for _, output := range outputs {
		if output != nil {
			allSpent = false
			// unspent outputs
			addrs, err := output.GetAddresses()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get output addresses: %w", err)
			}

			for _, address := range addrs {
				unspentTxOutputAddresses[address] = struct{}{}
			}
		}
	}

	dereferencedAddressed := []string{}

	spentOutputAddrs, err := spentOutput.GetAddresses()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get output addresses: %w", err)
	}

	for _, address := range spentOutputAddrs {
		if _, ok := unspentTxOutputAddresses[address]; !ok {
			// force to delete tx id lin for this address
			// because not referencing anymore
			dereferencedAddressed = append(dereferencedAddressed, address)
		}
	}

	// There is no left unpent outputs
	if allSpent {
		err := u.s.Delete(txOutputsKey.String())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to delete transaction id key: %w", err)
		}
	} else {
		err := u.s.Set(txOutputsKey.String(), outputs)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to set transaction id key: %w", err)
		}
	}

	return dereferencedAddressed, &spentOutput, nil
}

func (u *Store[T]) UpgradeTransactionOutputs(
	ctx context.Context,
	txID string,
	newOutputs []*TransactionOutput,
) error {
	err := u.setNewTxOutputs(txID, newOutputs)
	if err != nil {
		return err
	}

	return nil
}

func (u *Store[T]) AreExiststsOutputs(
	ctx context.Context,
	txID string,
) (bool, error) {
	var outputs []any
	var txOutputsKey = newTransactionIDKey(u.dbVer, txID)

	found, err := u.s.Get(txOutputsKey.String(), &outputs)
	if err != nil {
		return false, fmt.Errorf("failed to get transaction outputs: %w", err)
	}

	return found, nil
}

func (u *Store[T]) AddTransactionOutputs(
	ctx context.Context,
	txID string,
	outputs []*TransactionOutput,
) error {
	err := u.setNewTxOutputs(txID, outputs)
	if err != nil {
		return err
	}

	err = u.createAddressUTXOTxIdIndex(txID, outputs)
	if err != nil {
		return err
	}

	return nil
}

func (u *Store[T]) setNewTxOutputs(txID string, outputs []*TransactionOutput) error {
	txOutputsKey := newTransactionIDKey(u.dbVer, txID)

	err := u.s.Set(txOutputsKey.String(), outputs)
	if err != nil {
		return fmt.Errorf("failed to store tx outputs: %w", err)
	}

	return nil
}

func (u *Store[T]) createAddressUTXOTxIdIndex(txID string, outputs []*TransactionOutput) error {
	txIDsByAdddress := map[string][]string{}
	for _, output := range outputs {
		if output == nil {
			// spent
			continue
		}

		addrs, err := output.GetAddresses()
		if err != nil {
			return fmt.Errorf("failed to get output addresses: %w", err)
		}

		for _, address := range addrs {
			txIDsByAdddress[address] = append(txIDsByAdddress[address], txID)
		}
	}

	for address, txIDs := range txIDsByAdddress {
		err := u.addressUTXOIds.addAddressUTXOTransactionIds(address, txIDs)
		if err != nil {
			return err
		}
	}

	return nil
}

func isSpentOutput(output *TransactionOutput) bool {
	return output == nil
}

package utxostore

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
	redistx "github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/drivers/redis"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/samber/lo"
)

type CheckpointStore interface {
	// LastCheckpopintmust return latest checkpoint saved to the storage
	LastCheckpoint(ctx context.Context) (bool, *UTXOSpendingCheckpoint, error)

	// SaveCheckpoint must save new checkpoint and make it as the latest
	SaveCheckpoint(ctx context.Context, checkpoint *UTXOSpendingCheckpoint) error

	// RemoveLatestCheckpoint must remove latest checkpoint
	RemoveLatestCheckpoint(ctx context.Context) error
}

type Store[T any] struct {
	s    keyvaluestore.StoreWithTxManager[T]
	sets sets.SetsWithTxManager[T]

	txManager *txmanager.TransactionManager[T]

	checkpointsStore CheckpointStore
	isConsist        bool
	fixConsistencyOp chan struct{}

	addressUTXOIds *redisAddressUTXOIdx[T]

	// dbVer storing the version of the database (prefix for all keys)
	dbVer string
}

func New[T any](
	databaseVersion string,
	storer keyvaluestore.StoreWithTxManager[T],
	setsStore sets.SetsWithTxManager[T],
	txManager *txmanager.TransactionManager[T],
	checkpointStore CheckpointStore,
) (*Store[T], error) {
	store := &Store[T]{
		s:              storer,
		addressUTXOIds: newAddressUTXOIndex(databaseVersion, setsStore),
		dbVer:          databaseVersion,
		sets:           setsStore,
		txManager:      txManager,

		checkpointsStore: checkpointStore,
		isConsist:        false,
		fixConsistencyOp: make(chan struct{}, 1),
	}

	err := store.checkConsistencyAndTryToFix(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to check consistency: %w", err)
	}

	return store, nil
}

func (u *Store[T]) checkConsistencyAndTryToFix(ctx context.Context) error {
	if u.isConsist {
		return nil
	}

	defer func() {
		<-u.fixConsistencyOp
	}()

	u.fixConsistencyOp <- struct{}{}
	if u.isConsist {
		return nil
	}

	exists, latestCheckpoint, err := u.checkpointsStore.LastCheckpoint(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest checkpoint: %w", err)
	}

	if !exists {
		u.isConsist = true
		return nil
	}

	ok, err := u.isConsistenceWithCheckpoint(ctx, latestCheckpoint)
	if err != nil {
		return fmt.Errorf("failed to check store consistency with checkpoint")
	}

	if ok {
		err = u.checkpointsStore.RemoveLatestCheckpoint(ctx)
		if err != nil {
			return fmt.Errorf("failed to remove last checkpoint: %w", err)
		}

		u.isConsist = true
		return nil
	}

	// fix consistency
	err = u.downCheckpoint(ctx, latestCheckpoint)
	if err != nil {
		return fmt.Errorf("failed to downgrade to previouse state before checkpoint: %w", err)
	}

	err = u.checkpointsStore.RemoveLatestCheckpoint(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove checkpoint")
	}

	return nil
}

func (u *Store[T]) downCheckpoint(
	ctx context.Context,
	checkpoint *UTXOSpendingCheckpoint,
) error {
	previousBlockHash := checkpoint.GetPreviousBlockHash()
	previousBlockHeight := checkpoint.GetPreviousBlockheight()

	err := u.setBlockHash(ctx, previousBlockHash)
	if err != nil {
		return fmt.Errorf("failed to restore block hash: %w", err)
	}

	err = u.setBlockHeight(ctx, previousBlockHeight)
	if err != nil {
		return fmt.Errorf("failed to restore block height: %w", err)
	}

	newAddressReferences := checkpoint.GetNewAddreessReferences()
	for addr, txIDs := range newAddressReferences {
		err = u.removeAddressTxIDS(ctx, addr, txIDs)
		if err != nil {
			return fmt.Errorf("failed to remove address tx ids: %w", err)
		}
	}

	dereferencedAddresses := checkpoint.GetDereferencedAddressesTxs()
	for addr, txIDs := range dereferencedAddresses {
		err = u.addressUTXOIds.addAddressUTXOTransactionIds(ctx, addr, txIDs)
		if err != nil {
			return fmt.Errorf("failed to add dereferenced addres tx ids: %w", err)
		}
	}

	txOutputsBeforeUpdate := checkpoint.GetTransactionsBeforeUpdate()
	newTxOutputs := checkpoint.GetNewTransactionsOutputs()

	for txID := range newTxOutputs {
		if _, ok := txOutputsBeforeUpdate[txID]; !ok {
			err = u.spendAllOutputs(ctx, txID)
			if err != nil {
				return fmt.Errorf("failed to remove new tx outputs: %w", err)
			}
		}
	}

	for txID, outputs := range txOutputsBeforeUpdate {
		err = u.setTransactionOutputs(ctx, txID, outputs)
		if err != nil {
			return fmt.Errorf("failed to restore transaction outputs: %w", err)
		}
	}

	return nil
}

func (u *Store[T]) isConsistenceWithCheckpoint(
	ctx context.Context,
	checkpoint *UTXOSpendingCheckpoint,
) (bool, error) {

	currentBlockHash, err := u.getBlockHash(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get current block hash: %w", err)
	}

	if currentBlockHash != checkpoint.GetNextBlockHash() {
		return false, nil
	}

	currentBlockHeight, err := u.getBlockHeight(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get current block height: %w", err)
	}

	if currentBlockHeight != checkpoint.GetNewBlockHeight() {
		return false, nil
	}

	for addr, txIDS := range checkpoint.NewAddreessReferences {
		currrentTxIDS, err := u.addressUTXOIds.getAddressUTXOTransactionIds(ctx, addr)
		if err != nil {
			return false, fmt.Errorf("failed to get address transaction ids: %w", err)
		}

		if !lo.Every(currrentTxIDS, txIDS) {
			return false, nil
		}
	}

	for addr, txIDS := range checkpoint.DereferencedAddressesTxs {
		currentTxIDS, err := u.addressUTXOIds.getAddressUTXOTransactionIds(ctx, addr)
		if err != nil {
			return false, fmt.Errorf("failed to get address transaction ids: %w", err)
		}

		if !lo.None(currentTxIDS, txIDS) {
			return false, nil
		}
	}

	for txID, outputs := range checkpoint.GetNewTransactionsOutputs() {
		if isAllSpent(outputs) {
			_, err = u.getOutputsByTxID(ctx, txID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					continue
				}

				return false, fmt.Errorf("failed to get tx outputs: %w", err)
			}

			return false, nil
		} else {
			currentOutputs, err := u.getOutputsByTxID(ctx, txID)
			if err != nil {
				return false, fmt.Errorf("failed to get transaction outputs: %w", err)
			}

			if !reflect.DeepEqual(currentOutputs, outputs) {
				return false, nil
			}
		}
	}

	return true, nil
}

func isAllSpent(outputs []*TransactionOutput) bool {
	for _, output := range outputs {
		if !isSpentOutput(output) {
			return false
		}
	}

	return true
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
		s:                storerWithTx,
		addressUTXOIds:   newAddressUTXOIndex(u.dbVer, setsWithTx),
		dbVer:            u.dbVer,
		txManager:        u.txManager,
		isConsist:        u.isConsist,
		fixConsistencyOp: u.fixConsistencyOp,
	}, nil
}

func (u *Store[T]) Flush(ctx context.Context) error {
	if err := u.s.DeleteByPattern(ctx, u.dbVer+":*"); err != nil {
		return fmt.Errorf("flush error: %w", err)
	}

	return nil
}

func (u *Store[T]) GetBlockHeight(ctx context.Context) (int64, error) {
	if !u.isConsist {
		return 0, ErrInconsistenceState
	}

	return u.getBlockHeight(ctx)
}

func (u *Store[T]) getBlockHeight(ctx context.Context) (int64, error) {
	blockHeightKey := newBlockHeightKey(u.dbVer)

	var currentBlockHeight int64

	_, err := u.s.Get(ctx, blockHeightKey.String(), &currentBlockHeight)
	if err != nil {
		return 0, fmt.Errorf("failed to get current block height: %w", err)
	}

	return currentBlockHeight, nil
}

func (u *Store[T]) GetBlockHash(ctx context.Context) (string, error) {
	if !u.isConsist {
		return "", ErrInconsistenceState
	}

	return u.getBlockHash(ctx)
}

func (u *Store[T]) getBlockHash(ctx context.Context) (string, error) {
	blockHashKey := newBlockHashKey(u.dbVer)

	var currentBlockHash string

	found, err := u.s.Get(ctx, blockHashKey.String(), &currentBlockHash)
	if err != nil {
		return "", fmt.Errorf("failed to get current block hash: %w", err)
	}

	if !found {
		return "", ErrBlockHashNotFound
	}

	return currentBlockHash, nil
}

func (u *Store[T]) SetBlockHeight(ctx context.Context, blockHeight int64) error {
	if !u.isConsist {
		return ErrInconsistenceState
	}

	return u.setBlockHeight(ctx, blockHeight)
}

func (u *Store[T]) setBlockHeight(ctx context.Context, blockHeight int64) error {
	blockHeightKey := newBlockHeightKey(u.dbVer)

	err := u.s.Set(ctx, blockHeightKey.String(), blockHeight)
	if err != nil {
		return fmt.Errorf("failed to store new block height: %w", err)
	}

	return nil
}

func (u *Store[T]) SetBlockHash(ctx context.Context, blockHash string) error {
	if !u.isConsist {
		return ErrInconsistenceState
	}

	return u.setBlockHash(ctx, blockHash)
}

func (u *Store[T]) setBlockHash(ctx context.Context, blockHash string) error {
	blockHashKey := newBlockHashKey(u.dbVer)

	err := u.s.Set(ctx, blockHashKey.String(), blockHash)
	if err != nil {
		return fmt.Errorf("failed to store new block hash: %w", err)
	}

	return nil
}

func (u *Store[T]) RemoveAddressTxIDs(ctx context.Context, address string, txIDs []string) error {
	if !u.isConsist {
		return ErrInconsistenceState
	}

	return u.removeAddressTxIDS(ctx, address, txIDs)
}

func (u *Store[T]) removeAddressTxIDS(ctx context.Context, address string, txIDs []string) error {
	if err := u.addressUTXOIds.deleteAdressUTXOTransactionIds(ctx, address, txIDs); err != nil {
		return fmt.Errorf("delete address UTXO tx ids error: %w", err)
	}

	return nil
}

func (u *Store[T]) GetUnspentOutputsByAddress(ctx context.Context, address string) ([]*UTXOEntry, error) {
	if !u.isConsist {
		return nil, ErrInconsistenceState
	}

	return u.getUnspentOutputsByAddress(ctx, address)
}

func (u *Store[T]) getUnspentOutputsByAddress(
	ctx context.Context,
	address string,
) ([]*UTXOEntry, error) {
	txIDsWithOutputs, err := u.addressUTXOIds.getAddressUTXOTransactionIds(ctx, address)
	if err != nil {
		return nil, err
	}

	if len(txIDsWithOutputs) == 0 {
		return nil, nil
	}

	txIDKeys := make([]string, 0, len(txIDsWithOutputs))
	txIDKeysFormatted := map[string]string{}

	for _, txID := range txIDsWithOutputs {
		formattedTxID := newTransactionIDKey(u.dbVer, txID).String()
		txIDKeys = append(txIDKeys, formattedTxID)

		txIDKeysFormatted[formattedTxID] = txID
	}

	type outputsEntry struct {
		txID    string
		outputs []*TransactionOutput
	}

	outputsEntries := []*outputsEntry{}

	if err := u.s.MulGet(ctx, func(ctx context.Context, key string) any {
		entry := outputsEntry{
			txID:    txIDKeysFormatted[key],
			outputs: []*TransactionOutput{},
		}

		outputsEntries = append(outputsEntries, &entry)

		return &entry.outputs
	}, txIDKeys...); err != nil {
		return nil, fmt.Errorf("failed to get tx outputs: %w", err)
	}

	if len(outputsEntries) == 0 {
		return nil, nil
	}

	res := []*UTXOEntry{}

	for _, entry := range outputsEntries {
		txID := entry.txID
		outputs := entry.outputs

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
	ctx context.Context,
	txID string,
) ([]*TransactionOutput, error) {
	if !u.isConsist {
		return nil, ErrInconsistenceState
	}

	return u.getOutputsByTxID(ctx, txID)
}

func (u *Store[T]) getOutputsByTxID(
	ctx context.Context,
	txID string,
) ([]*TransactionOutput, error) {
	var outputs []*TransactionOutput

	ok, err := u.s.Get(ctx, newTransactionIDKey(u.dbVer, txID).String(), &outputs)
	if err != nil {
		return nil, fmt.Errorf("failed to get outputs: %w", err)
	}

	if !ok {
		return nil, ErrNotFound
	}

	return outputs, nil
}

func (u *Store[T]) SpendOutputFromRetrievedOutputs(
	ctx context.Context,
	txID string,
	outputs []*TransactionOutput,
	idx int,
) ([]string, *TransactionOutput, error) {
	if !u.isConsist {
		return nil, nil, ErrInconsistenceState
	}

	return u.spendOutputFromList(ctx, txID, idx, outputs)
}

func (u *Store[T]) SpendAllOutputs(
	ctx context.Context,
	txID string,
) error {
	if !u.isConsist {
		return ErrInconsistenceState
	}

	return u.spendAllOutputs(ctx, txID)
}

func (u *Store[T]) spendAllOutputs(
	ctx context.Context,
	txID string,
) error {
	var txOutputsKey = newTransactionIDKey(u.dbVer, txID)
	if err := u.s.Delete(ctx, txOutputsKey.String()); err != nil {
		return fmt.Errorf("delete outputs error: %w", err)
	}

	return nil
}

func (u *Store[T]) SpendOutput(
	ctx context.Context,
	txID string,
	idx int,
) ([]string, *TransactionOutput, error) {
	if !u.isConsist {
		return nil, nil, ErrInconsistenceState
	}

	return u.spendOutput(ctx, txID, idx)
}

func (u *Store[T]) spendOutput(
	ctx context.Context,
	txID string,
	idx int,
) ([]string, *TransactionOutput, error) {
	var outputs []*TransactionOutput
	var txOutputsKey = newTransactionIDKey(u.dbVer, txID)

	found, err := u.s.Get(ctx, txOutputsKey.String(), &outputs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get transaction outputs: %w", err)
	}

	if !found {
		return nil, nil, ErrNotFound
	}

	return u.spendOutputFromList(ctx, txID, idx, outputs)
}

func (u *Store[T]) spendOutputFromList(
	ctx context.Context,
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
		err := u.s.Delete(ctx, txOutputsKey.String())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to delete transaction id key: %w", err)
		}
	} else {
		err := u.s.Set(ctx, txOutputsKey.String(), outputs)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to set transaction id key: %w", err)
		}
	}

	return dereferencedAddressed, &spentOutput, nil
}

func (u *Store[T]) SetTransactionOutputs(
	ctx context.Context,
	txID string,
	newOutputs []*TransactionOutput,
) error {
	if !u.isConsist {
		return ErrInconsistenceState
	}

	return u.setTransactionOutputs(ctx, txID, newOutputs)
}

func (u *Store[T]) setTransactionOutputs(
	ctx context.Context,
	txID string,
	newOutputs []*TransactionOutput,
) error {
	err := u.setNewTxOutputs(ctx, txID, newOutputs)
	if err != nil {
		return err
	}

	return nil
}

func (u *Store[T]) CommitCheckpoint(
	ctx context.Context,
	checkpoint *UTXOSpendingCheckpoint,
) error {
	if !u.isConsist {
		return ErrInconsistenceState
	}

	err := u.checkpointsStore.SaveCheckpoint(ctx, checkpoint)
	if err != nil {
		return fmt.Errorf("failed to save checkppoint before update: %w", err)
	}

	// Before commit new checkpoint, we need to check the consistency of the current store state
	err = u.spendOutputsWithNewBlockInfo(
		ctx,
		checkpoint.GetNextBlockHash(),
		checkpoint.GetNewBlockHeight(),
		checkpoint.GetNewTransactionsOutputs(),
		checkpoint.GetDereferencedAddressesTxs(),
		checkpoint.GetNewAddreessReferences(),
	)
	if err != nil {
		return err
	}

	err = u.checkpointsStore.RemoveLatestCheckpoint(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove latest checkpoint: %w", err)
	}

	return nil
}

func (u *Store[T]) spendOutputsWithNewBlockInfo(
	ctx context.Context,
	newBlockHash string,
	newBlockHeight int64,
	newTxOutputs map[string][]*TransactionOutput,
	dereferencedAddresses map[string][]string,
	newAddressReferences map[string][]string,
) error {

	watchKeys := u.getStoreKeysToWatchFromCheckpointUP(
		lo.Keys(newTxOutputs),
		lo.Keys(dereferencedAddresses),
		lo.Keys(newAddressReferences),
	)

	err := u.txManager.Do(ctx, redistx.NewSettings(
		redistx.WithWatchKeys(watchKeys...),
	), func(ctx context.Context, tx txmanager.Transaction[T]) error {

		selfWithTx, err := u.WithTx(tx)
		if err != nil {
			return fmt.Errorf("failed to wrap store in transaction: %w", err)
		}

		err = selfWithTx.setBlockHash(ctx, newBlockHash)
		if err != nil {
			return fmt.Errorf("failed to set block height: %w", err)
		}

		err = selfWithTx.setBlockHeight(ctx, newBlockHeight)
		if err != nil {
			return fmt.Errorf("failed to set block height: %w", err)
		}

		for txID, outputs := range newTxOutputs {
			allSpent := true
			for _, output := range outputs {
				if !isSpentOutput(output) {
					allSpent = false
					break
				}
			}

			if allSpent {
				selfWithTx.spendAllOutputs(ctx, txID)
			} else {
				selfWithTx.setTransactionOutputs(ctx, txID, outputs)
			}
		}

		for address, txIDRefs := range newAddressReferences {
			err = selfWithTx.addressUTXOIds.addAddressUTXOTransactionIds(ctx, address, txIDRefs)
			if err != nil {
				return fmt.Errorf("failed to add address tx ids: %w", err)
			}
		}

		for address, dereferencedTxIDs := range dereferencedAddresses {
			err = selfWithTx.addressUTXOIds.deleteAdressUTXOTransactionIds(ctx, address, dereferencedTxIDs)
			if err != nil {
				return fmt.Errorf("failed to remove adddress tx id references")
			}
		}

		return nil
	})

	if err != nil {
		u.isConsist = false
		return fmt.Errorf("failed to comlete transaction: %w", err)
	}

	return nil
}

func (u *Store[T]) getStoreKeysToWatchFromCheckpointUP(
	changingTxIDS []string,
	dereferencedAddresses []string,
	newAddressReferences []string,
) []string {
	watchKeys := []string{}

	watchKeys = append(watchKeys, newBlockHeightKey(u.dbVer).String(), newBlockHeightKey(u.dbVer).String())
	watchKeys = append(watchKeys, newTransactionOutputsStringKeys(u.dbVer, changingTxIDS...)...)
	watchKeys = append(watchKeys, newAddressStringKeys(u.dbVer, dereferencedAddresses...)...)
	watchKeys = append(watchKeys, newAddressStringKeys(u.dbVer, newAddressReferences...)...)

	return watchKeys
}

func newTransactionOutputsStringKeys(dbVer string, txIDs ...string) []string {
	keys := make([]string, 0, len(txIDs))
	for _, txID := range txIDs {
		keys = append(keys, newTransactionIDKey(dbVer, txID).String())
	}

	return keys
}

func newAddressStringKeys(dbVer string, addresses ...string) []string {
	keys := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		keys = append(keys, newAddressUTXOTxIDsSetKey(dbVer, addr).String())
	}

	return keys
}

func (u *Store[T]) AreExiststsOutputs(
	ctx context.Context,
	txID string,
) (bool, error) {
	if !u.isConsist {
		return false, ErrInconsistenceState
	}

	return u.areExistsOutputs(ctx, txID)
}

func (u *Store[T]) areExistsOutputs(
	ctx context.Context,
	txID string,
) (bool, error) {
	var outputs []any
	var txOutputsKey = newTransactionIDKey(u.dbVer, txID)

	found, err := u.s.Get(ctx, txOutputsKey.String(), &outputs)
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
	if !u.isConsist {
		return ErrInconsistenceState
	}

	return u.addTransactionOutputs(ctx, txID, outputs)
}

func (u *Store[T]) addTransactionOutputs(
	ctx context.Context,
	txID string,
	outputs []*TransactionOutput,
) error {
	err := u.setNewTxOutputs(ctx, txID, outputs)
	if err != nil {
		return err
	}

	err = u.createAddressUTXOTxIdIndex(ctx, txID, outputs)
	if err != nil {
		return err
	}

	return nil
}

func (u *Store[T]) setNewTxOutputs(ctx context.Context, txID string, outputs []*TransactionOutput) error {
	txOutputsKey := newTransactionIDKey(u.dbVer, txID)

	err := u.s.Set(ctx, txOutputsKey.String(), outputs)
	if err != nil {
		return fmt.Errorf("failed to store tx outputs: %w", err)
	}

	return nil
}

func (u *Store[T]) createAddressUTXOTxIdIndex(ctx context.Context, txID string, outputs []*TransactionOutput) error {
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
		err := u.addressUTXOIds.addAddressUTXOTransactionIds(ctx, address, txIDs)
		if err != nil {
			return err
		}
	}

	return nil
}

func isSpentOutput(output *TransactionOutput) bool {
	return output == nil
}

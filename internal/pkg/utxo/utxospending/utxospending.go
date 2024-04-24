package utxospending

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
)

type UTXOStoreReadMethods interface {
	GetOutputsByTxID(_ context.Context, txID string) (*utxostore.TransactionOutputs, error)
	GetBlockHeight(_ context.Context) (int64, error)
	GetBlockHash(_ context.Context) (string, error)
}

type BitcoinConfig interface {
	GetDecimals() int
}

type UTXOSpender struct {
	store     UTXOStoreReadMethods
	btcConfig BitcoinConfig
}

func NewUTXOSpender(store UTXOStoreReadMethods, btcConfig BitcoinConfig) (*UTXOSpender, error) {
	return &UTXOSpender{
		store:     store,
		btcConfig: btcConfig,
	}, nil
}

func (u *UTXOSpender) NewCheckpointFromNextBlock(
	ctx context.Context,
	newBlock *blockchain.Block,
) (*utxostore.UTXOSpendingCheckpoint, error) {
	var prevBlock *utxostore.CheckpointBlock

	currentHash, err := u.store.GetBlockHash(ctx)
	if err != nil {
		if !errors.Is(err, utxostore.ErrBlockHashNotFound) {
			return nil, fmt.Errorf("failed to get current block hash: %w", err)
		}
	} else {
		currentHeight, err := u.store.GetBlockHeight(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get current block height: %w", err)
		}

		if newBlock.GetHeight() == currentHeight {
			return nil, ErrNoDiff
		}

		if currentHeight+1 != newBlock.GetHeight() {
			return nil, ErrNextBlockTooFar
		}

		prevBlock = &utxostore.CheckpointBlock{
			Hash:   currentHash,
			Height: currentHeight,
		}
	}

	newBlockPoint := &utxostore.CheckpointBlock{
		Height: newBlock.GetHeight(),
		Hash:   newBlock.GetHash().String(),
	}

	if prevBlock == nil {
		// we can't spend any output form the genesis block (only Bitcoin for now)
		return utxostore.NewUTXOSpendingCheckpoint(
			nil,
			newBlockPoint,
			map[string]*utxostore.TransactionOutputs{},
			map[string][]string{}, map[string]*utxostore.TransactionOutputs{}, map[string][]string{},
		), nil
	}

	// collecting all spending outputs and their current txs outputs state
	spendingOutputs, err := u.extractSpendingOutputsFromBlock(ctx, newBlock)
	if err != nil {
		return nil, err
	}

	//  collecting all addresses and txs from which they are dereferenced
	dereferencedAddresses, err := derefAddresses(spendingOutputs, newBlock)
	if err != nil {
		return nil, err
	}

	// spending all outputs and creating a new map of txs outputs
	newTxOutputs, err := spendAllOutputs(u.btcConfig, spendingOutputs, newBlock)
	if err != nil {
		return nil, err
	}

	// collecting new addresses references
	newAddreessReferences, err := extractAddressReferencesFromOutputs(u.btcConfig, newBlock)
	if err != nil {
		return nil, err
	}

	return utxostore.NewUTXOSpendingCheckpoint(
		prevBlock,
		newBlockPoint,
		spendingOutputs,
		dereferencedAddresses,
		newTxOutputs,
		newAddreessReferences,
	), nil
}

func extractAddressReferencesFromOutputs(
	btcConfig BitcoinConfig,
	newBlock *blockchain.Block,
) (map[string][]string, error) {
	addressRefs := map[string]map[string]bool{}
	for _, tx := range newBlock.GetTransactions() {
		spendableOutputs, err := getAllSpendableOutputs(btcConfig, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to cast transaction outputs to store outputs: %w", err)
		}

		for _, output := range spendableOutputs {
			if output == nil {
				continue
			}

			addresses, err := output.GetAddresses()
			if err != nil {
				return nil, fmt.Errorf("failed to get outputs addresses: %w", err)
			}

			for _, addr := range addresses {
				if _, ok := addressRefs[addr]; !ok {
					addressRefs[addr] = map[string]bool{}
				}

				addressRefs[addr][tx.GetID()] = true
			}

		}
	}

	addressRefsList := map[string][]string{}
	for addr, txs := range addressRefs {
		for txID := range txs {
			addressRefsList[addr] = append(addressRefsList[addr], txID)
		}
	}

	return addressRefsList, nil
}

func spendAllOutputs(
	btcConfig BitcoinConfig,
	spendingOutputs map[string]*utxostore.TransactionOutputs,
	newBlock *blockchain.Block,
) (map[string]*utxostore.TransactionOutputs, error) {
	newTxOutputs := map[string]*utxostore.TransactionOutputs{}

	for txID, txOutputsEntry := range spendingOutputs {
		newOutputs := utxostore.TransactionOutputs{
			BlockHeight: txOutputsEntry.BlockHeight,
			Outputs:     make([]*utxostore.TransactionOutput, len(txOutputsEntry.Outputs)),
		}

		copy(newOutputs.Outputs, txOutputsEntry.Outputs)

		newTxOutputs[txID] = &newOutputs
	}

	for _, tx := range newBlock.GetTransactions() {
		spendableOutputs, err := getAllSpendableOutputs(btcConfig, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to cast transaction outputs to store outputs: %w", err)
		}

		newTxOutputs[tx.GetID()] = &utxostore.TransactionOutputs{
			BlockHeight: newBlock.GetHeight(),
			Outputs:     spendableOutputs,
		}

		for _, input := range tx.GetInputs() {
			// spending all outputs
			if input.SpendingOutput == nil {
				continue
			}

			spendingOutput := input.SpendingOutput

			txOutputsEntry, ok := newTxOutputs[spendingOutput.GetTxID()]
			if !ok {
				return nil, ErrSpendingOutputNotFound
			}

			if len(txOutputsEntry.Outputs) < spendingOutput.VOut+1 {
				return nil, ErrSpendingOutputNotFound
			}

			txOutputsEntry.Outputs[spendingOutput.VOut] = nil
		}
	}

	return newTxOutputs, nil
}

func getAllSpendableOutputs(btcConfig BitcoinConfig, tx *blockchain.Transaction) ([]*utxostore.TransactionOutput, error) {
	decimals := btcConfig.GetDecimals()

	outputs := tx.GetOutputs()
	if len(outputs) == 0 {
		return nil, nil
	}

	convertedOutputs := make([]*utxostore.TransactionOutput, len(outputs))
	for i, output := range outputs {
		if output.IsSpendable() {
			scriptBytes, err := hex.DecodeString(output.GetScriptPubKey().HEX)
			if err != nil {
				return nil, fmt.Errorf("failed to convert script to bytes: %w", err)
			}

			convertedOutput := &utxostore.TransactionOutput{}

			convertedOutput.SetScriptBytes(scriptBytes)

			if output.Value == nil {
				convertedOutput.SetAmount(0)
			} else {
				convertedOutput.SetAmount(output.Value.Uint64(decimals))
			}

			convertedOutputs[i] = convertedOutput

		} else {
			// is unspendable output, so, we push it as already "spent"
			convertedOutputs[i] = nil
		}
	}

	return convertedOutputs, nil
}

func countAddressOutputs(
	txOutputs map[string]*utxostore.TransactionOutputs,
) (map[string]map[string]int, error) {
	addressOutputsCount := map[string]map[string]int{}

	for txID, txOutputsEntry := range txOutputs {
		for _, output := range txOutputsEntry.Outputs {
			if output == nil {
				continue
			}

			addresses, err := output.GetAddresses()
			if err != nil {
				return nil, fmt.Errorf("failed to get output addresses: %w", err)
			}

			for _, addr := range addresses {
				if _, ok := addressOutputsCount[addr]; !ok {
					addressOutputsCount[addr] = map[string]int{}
				}

				addressOutputsCount[addr][txID]++
			}
		}
	}

	return addressOutputsCount, nil
}

func derefAddresses(
	spendingOutputs map[string]*utxostore.TransactionOutputs,
	newBlock *blockchain.Block,
) (map[string][]string, error) {
	addressesOutputsCount, err := countAddressOutputs(spendingOutputs)
	if err != nil {
		return nil, err
	}

	txIDsFromCurrentBlock := map[string]bool{}

	// we need to collect all dereferenced addresses after spending outputs
	for _, tx := range newBlock.GetTransactions() {
		txIDsFromCurrentBlock[tx.GetID()] = true

		for _, input := range tx.GetInputs() {
			if input.SpendingOutput == nil {
				continue
			}

			output := input.SpendingOutput
			if _, ok := txIDsFromCurrentBlock[output.GetTxID()]; ok {
				continue
			}

			txOutputsEntry, ok := spendingOutputs[output.GetTxID()]
			if !ok {
				return nil, ErrSpendingOutputNotFound
			}

			if len(txOutputsEntry.Outputs) < output.VOut+1 {
				return nil, ErrSpendingOutputNotFound
			}

			spentOutput := txOutputsEntry.Outputs[output.VOut]

			addresses, err := spentOutput.GetAddresses()
			if err != nil {
				return nil, fmt.Errorf("failed to get output addresses: %w", err)
			}

			for _, addr := range addresses {
				addressesOutputsCount[addr][output.GetTxID()]--
			}
		}
	}

	dereferencedAddressTxs := map[string][]string{}
	for address, txOuputs := range addressesOutputsCount {
		for txID, outputsCount := range txOuputs {
			if outputsCount == 0 {
				dereferencedAddressTxs[address] = append(dereferencedAddressTxs[address], txID)
			}
		}
	}

	return dereferencedAddressTxs, nil
}

func (u *UTXOSpender) extractSpendingOutputsFromBlock(
	ctx context.Context,
	block *blockchain.Block,
) (map[string]*utxostore.TransactionOutputs, error) {
	outputs := map[string]*utxostore.TransactionOutputs{}
	txIDsFromCurrentBlock := map[string]bool{}

	for _, tx := range block.GetTransactions() {
		txIDsFromCurrentBlock[tx.GetID()] = true
		for _, input := range tx.GetInputs() {
			if input.SpendingOutput != nil {
				txID := input.SpendingOutput.GetTxID()

				// if the transaction id in the same block, we need to ignore not found error
				spendingOutputs, err := u.store.GetOutputsByTxID(ctx, txID)
				if err != nil {
					if errors.Is(err, utxostore.ErrNotFound) {
						if _, ok := txIDsFromCurrentBlock[txID]; ok {
							continue
						}
					}

					return nil, fmt.Errorf("get tx outputs error: %w", err)
				}

				outputs[txID] = spendingOutputs
			}
		}
	}

	return outputs, nil
}

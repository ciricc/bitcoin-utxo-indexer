package utxoservice

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
)

type UTXOStore interface {
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	GetOutputsByTxID(_ context.Context, txID string) ([]*utxostore.TransactionOutput, error)
	SpendOutput(_ context.Context, txID string, idx int) ([]string, *utxostore.TransactionOutput, error)
	WithStorer(storer keyvaluestore.Store, sets sets.Sets) *utxostore.Store
	GetUnspentOutputsByAddress(_ context.Context, address string) ([]*utxostore.UTXOEntry, error)
	GetBlockHeight(_ context.Context) (int64, error)
	GetBlockHash(_ context.Context) (string, error)
}

type ServiceOptions struct {
	Logger *zerolog.Logger
}

type Service[T any] struct {
	s UTXOStore

	txManager *txmanager.TransactionManager[T]
	txStore   keyvaluestore.StoreWithTxManager[T]
	txSets    sets.SetsWithTxManager[T]
	btcConfig BitcoinConfig

	// logger is the logger used by the service.
	logger *zerolog.Logger
}

type BitcoinConfig interface {
	GetDecimals() int
}

func New[T any](
	s UTXOStore,

	txManager *txmanager.TransactionManager[T],
	utxoKVStore keyvaluestore.StoreWithTxManager[T],
	sets sets.SetsWithTxManager[T],
	bitcoinConfig BitcoinConfig,

	options *ServiceOptions,
) *Service[T] {
	defaultOptions := &ServiceOptions{
		Logger: zerolog.DefaultContextLogger,
	}

	if options != nil {
		if options.Logger != nil {
			defaultOptions.Logger = options.Logger
		}
	}

	return &Service[T]{
		s:         s,
		txStore:   utxoKVStore,
		txSets:    sets,
		logger:    defaultOptions.Logger,
		txManager: txManager,
	}
}

func (u *Service[T]) GetBlockHash(ctx context.Context) (string, error) {
	hash, err := u.s.GetBlockHash(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get block hash from store: %w", err)
	}

	return hash, nil
}

func (u *Service[T]) GetBlockHeight(ctx context.Context) (int64, error) {
	height, err := u.s.GetBlockHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get block height from store: %w", err)
	}

	return height, nil
}

func (u *Service[T]) GetUTXOByAddress(ctx context.Context, address string) ([]*utxostore.UTXOEntry, error) {
	outputs, err := u.s.GetUnspentOutputsByAddress(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get UTXO by address: %w", err)
	}

	return outputs, nil
}

func (u *Service[T]) AddFromBlock(ctx context.Context, block *blockchain.Block) error {
	return u.txManager.Do(func(ctx context.Context, tx txmanager.Transaction[T]) error {
		currentHeight, err := u.s.GetBlockHeight(ctx)
		if err != nil {
			return err
		}

		if currentHeight >= block.GetHeight() && currentHeight != 0 {
			return ErrBlockAlreadyStored
		}

		// First, we need to complete all "GET" commands before we can start "SET" commands
		// in the transaction
		spendingOutputs, err := u.getSpeningOutputs(ctx, block)
		if err != nil {
			return err
		}

		storeWithTx, err := u.txStore.WithTx(tx)
		if err != nil {
			return err
		}

		setsWithTx, err := u.txSets.WithTx(tx)
		if err != nil {
			return err
		}

		utxoStoreWithTx := u.s.WithStorer(storeWithTx, setsWithTx)

		// Updating block height
		if err := utxoStoreWithTx.SetBlockHeight(ctx, block.GetHeight()); err != nil {
			if errors.Is(err, utxostore.ErrIsPreviousBlockHeight) {
				return ErrBlockAlreadyStored
			}

			return err
		}

		if err := utxoStoreWithTx.SetBlockHash(ctx, block.GetHash().String()); err != nil {
			return err
		}

		dereferenceAddresseByTxIDs := map[string][]string{}

		for _, tx := range block.GetTransactions() {

			convertedOutputs, err := getTransactionsOutputsForStore(u.btcConfig, tx)
			if err != nil {
				return err
			}

			err = utxoStoreWithTx.AddTransactionOutputs(
				ctx,
				tx.GetID(),
				convertedOutputs,
			)
			if err != nil {
				return fmt.Errorf("failed to store utxo: %w", err)
			}

			dereferencedAddresses, err := u.spendOutputs(ctx, spendingOutputs, tx)
			if err != nil {
				return fmt.Errorf("failed to spend outputs: %w", err)
			}

			for address, txID := range dereferencedAddresses {
				dereferenceAddresseByTxIDs[address] = append(dereferenceAddresseByTxIDs[address], txID)
			}
		}

		// we need to store new transaction outputs and delete all tx ids if there is no
		// any output
		for txID, outputs := range spendingOutputs {
			allSpent := true

			for _, output := range outputs {
				if output != nil {
					allSpent = false
					break
				}
			}

			if allSpent {
				err := utxoStoreWithTx.SpendAllOutputs(ctx, txID)
				if err != nil {
					return fmt.Errorf("spend all outputs error: %w", err)
				}

			} else {
				err := utxoStoreWithTx.UpgradeTransactionOutputs(ctx, txID, outputs)
				if err != nil {
					return fmt.Errorf("upgrade tx outputs error: %w", err)
				}
			}
		}

		// after, we need to dereference all addresses not referencing anymore
		for address, txIDs := range dereferenceAddresseByTxIDs {
			err := utxoStoreWithTx.RemoveAddressTxIDs(ctx, address, txIDs)
			if err != nil {
				return fmt.Errorf("remove error: %w", err)
			}
		}

		return nil
	})
}

func (s *Service[T]) getSpeningOutputs(
	ctx context.Context,
	tx *blockchain.Block,
) (map[string][]*utxostore.TransactionOutput, error) {
	outputs := map[string][]*utxostore.TransactionOutput{}

	for _, tx := range tx.GetTransactions() {
		convertedOutputs, err := getTransactionsOutputsForStore(s.btcConfig, tx)
		if err != nil {
			return nil, err
		}

		outputs[tx.GetID()] = convertedOutputs

		for _, input := range tx.GetInputs() {
			if input.SpendingOutput != nil {
				txID := input.SpendingOutput.GetTxID()

				if _, ok := outputs[txID]; ok {
					continue
				}

				spendingOutputs, err := s.s.GetOutputsByTxID(ctx, txID)
				if err != nil {
					return nil, fmt.Errorf("get tx outputs error: %w", err)
				}

				outputs[txID] = spendingOutputs
			}
		}
	}

	return outputs, nil
}

// spendOutputs spend all in-memory outputs for the next to store
// the transaction outputs
// It returns the list of addresses which need to dereference form this transaction
// after spending
func (s *Service[T]) spendOutputs(
	ctx context.Context,
	storedOutputs map[string][]*utxostore.TransactionOutput,
	tx *blockchain.Transaction,
) (map[string]string, error) {

	// address => dereferenced tx id
	dereferencedAddresses := map[string]string{}

	for _, input := range tx.GetInputs() {
		if input.SpendingOutput != nil {

			spendingOutputs := storedOutputs[input.SpendingOutput.TxID]
			vout := input.SpendingOutput.VOut

			if len(spendingOutputs) <= vout {
				return nil, utxostore.ErrNotFound
			}

			if spendingOutputs[vout] == nil {
				return nil, utxostore.ErrAlreadySpent
			}

			spendingOutputAddresses, err := spendingOutputs[vout].GetAddresses()
			if err != nil {
				return nil, fmt.Errorf("failed to get output addresses: %w", err)
			}

			unspentAddresses := map[string]bool{}

			spendingOutputs[vout] = nil

			for _, output := range spendingOutputs {
				if output == nil {
					continue
				}

				addrs, err := output.GetAddresses()
				if err != nil {
					return nil, fmt.Errorf("failed to get output addresses: %w", err)
				}

				for _, address := range addrs {
					unspentAddresses[address] = true
				}
			}

			for _, address := range spendingOutputAddresses {
				if _, ok := unspentAddresses[address]; !ok {
					dereferencedAddresses[address] = input.SpendingOutput.TxID
				}
			}
		}
	}

	return dereferencedAddresses, nil
}

func getTransactionsOutputsForStore(btcConfig BitcoinConfig, tx *blockchain.Transaction) ([]*utxostore.TransactionOutput, error) {
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

			convertedOutput := &utxostore.TransactionOutput{
				ScriptBytes: scriptBytes,
			}

			convertedOutput.SetAmount(output.Value.Uint64(decimals))

			convertedOutputs[i] = convertedOutput

		} else {
			// is unspendable output, so, we push it as already "spent"
			convertedOutputs[i] = nil
		}
	}

	return convertedOutputs, nil
}

func getOutputAdresses(output *blockchain.TransactionOutput) []string {
	var addresses []string

	if len(output.ScriptPubKey.Addresses) != 0 {
		addresses = make([]string, len(output.ScriptPubKey.Addresses))

		copy(addresses, output.ScriptPubKey.Addresses)
	} else if output.ScriptPubKey.Address != "" {
		addresses = make([]string, 1)
		addresses[0] = output.ScriptPubKey.Address
	}

	return addresses
}

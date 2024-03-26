package utxoservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
)

type UTXOStore interface {
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	GetOutputsByTxID(_ context.Context, txID string) ([]*utxostore.TransactionOutput, error)
	SpendOutput(_ context.Context, txID string, idx int) (*utxostore.TransactionOutput, error)
	WithStorer(storer keyvaluestore.Store) *utxostore.Store
	GetUnspentOutputsByAddress(_ context.Context, address string) ([]*utxostore.TransactionOutput, error)
	GetBlockHeight(_ context.Context) (int64, error)
}

type ServiceOptions struct {
	Logger *zerolog.Logger
}

type Service[T any] struct {
	s UTXOStore

	txManager    *txmanager.TransactionManager[T]
	ldbUTXOStore keyvaluestore.StoreWithTxManager[T]

	// logger is the logger used by the service.
	logger *zerolog.Logger
}

func New[T any](
	s UTXOStore,

	txManager *txmanager.TransactionManager[T],
	utxoKVStore keyvaluestore.StoreWithTxManager[T],

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
		s:            s,
		ldbUTXOStore: utxoKVStore,
		logger:       defaultOptions.Logger,
		txManager:    txManager,
	}
}

func (u *Service[T]) GetBlockHeight(ctx context.Context) (int64, error) {
	height, err := u.s.GetBlockHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get block height from store: %w", err)
	}

	return height, nil
}

func (u *Service[T]) GetUTXOByAddress(ctx context.Context, address string) ([]bool, error) {
	outputs, err := u.s.GetUnspentOutputsByAddress(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get UTXO by address: %w", err)
	}

	if len(outputs) == 0 {
		return nil, nil
	}

	b := make([]bool, 0, len(outputs))
	for range outputs {
		b = append(b, true)
	}

	return b, nil
}

func (u *Service[T]) AddFromBlock(ctx context.Context, block *blockchain.Block) error {
	return u.txManager.Do(func(ctx context.Context, ldbTx txmanager.Transaction[T]) error {
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

		storeWithTx, err := u.ldbUTXOStore.WithTx(ldbTx)
		if err != nil {
			return err
		}

		utxoStoreWithTx := u.s.WithStorer(storeWithTx)

		// Updating block height
		if err := utxoStoreWithTx.SetBlockHeight(ctx, block.GetHeight()); err != nil {
			if errors.Is(err, utxostore.ErrIsPreviousBlockHeight) {
				return ErrBlockAlreadyStored
			}

			return err
		}

		for _, tx := range block.GetTransactions() {

			convertedOutputs := getTransactionsOutputsForStore(tx)
			err = utxoStoreWithTx.AddTransactionOutputs(
				ctx,
				tx.GetID(),
				convertedOutputs,
			)
			if err != nil {
				return fmt.Errorf("failed to store utxo: %w", err)
			}

			err = u.spendOutputs(ctx, spendingOutputs, utxoStoreWithTx, tx)
			if err != nil {
				return fmt.Errorf("failed to spend outputs: %w", err)
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
		outputs[tx.GetID()] = getTransactionsOutputsForStore(tx)

		for _, input := range tx.GetInputs() {
			if input.SpendingOutput != nil {
				txID := input.SpendingOutput.GetTxID()

				if _, ok := outputs[txID]; ok {
					continue
				}

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

func (s *Service[T]) spendOutputs(
	ctx context.Context,
	availableTxOutputs map[string][]*utxostore.TransactionOutput,
	utxoStore *utxostore.Store,
	tx *blockchain.Transaction,
) error {
	for _, input := range tx.GetInputs() {
		if input.SpendingOutput != nil {

			spendingOutputs := availableTxOutputs[input.SpendingOutput.TxID]

			_, err := utxoStore.SpendOutputFromRetrievedOutputs(ctx, input.SpendingOutput.GetTxID(), spendingOutputs, input.SpendingOutput.VOut)
			if err != nil {
				return fmt.Errorf("spend utxo error: %w", err)
			}
		}
	}

	return nil
}

func getTransactionsOutputsForStore(tx *blockchain.Transaction) []*utxostore.TransactionOutput {
	outputs := tx.GetOutputs()
	if len(outputs) == 0 {
		return nil
	}

	convertedOutputs := make([]*utxostore.TransactionOutput, len(outputs))
	for i, output := range outputs {
		convertedOutputs[i] = &utxostore.TransactionOutput{
			ScriptBytes: output.ScriptPubKey.HEX,
			Addresses:   getOutputAdresses(output),
			Amount:      output.Value.BigFloat,
		}
	}

	return convertedOutputs
}

func getOutputAdresses(output *blockchain.TransactionOutput) []string {
	var addresses []string

	if len(output.ScriptPubKey.Addresses) != 0 {
		addresses = make([]string, len(output.ScriptPubKey.Addresses))

		copy(output.ScriptPubKey.Addresses, addresses)
	} else if output.ScriptPubKey.Address != "" {
		addresses = make([]string, 1)
		addresses[0] = output.ScriptPubKey.Address
	}

	return addresses
}

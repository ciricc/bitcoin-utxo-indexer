package utxoservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/redis/go-redis/v9"
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

type Service struct {
	s UTXOStore

	txManager    *txmanager.TransactionManager[*redis.Tx]
	ldbUTXOStore keyvaluestore.StoreWithTxManager[*redis.Tx]

	// logger is the logger used by the service.
	logger *zerolog.Logger
}

func New(
	s UTXOStore,

	txManager *txmanager.TransactionManager[*redis.Tx],
	utxoKVStore keyvaluestore.StoreWithTxManager[*redis.Tx],

	options *ServiceOptions,
) *Service {
	defaultOptions := &ServiceOptions{
		Logger: zerolog.DefaultContextLogger,
	}

	if options != nil {
		if options.Logger != nil {
			defaultOptions.Logger = options.Logger
		}
	}

	return &Service{
		s:            s,
		ldbUTXOStore: utxoKVStore,
		logger:       defaultOptions.Logger,
		txManager:    txManager,
	}
}

func (u *Service) GetBlockHeight(ctx context.Context) (int64, error) {
	height, err := u.s.GetBlockHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get block height from store: %w", err)
	}

	return height, nil
}

func (u *Service) GetUTXOByAddress(ctx context.Context, address string) ([]bool, error) {
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

func (u *Service) AddFromBlock(ctx context.Context, block *blockchain.Block) error {
	return u.txManager.Do(func(ctx context.Context, ldbTx txmanager.Transaction[*redis.Tx]) error {
		storeWithTx, err := u.ldbUTXOStore.WithTx(ldbTx)
		if err != nil {
			return err
		}

		utxoStoreWithTx := u.s.WithStorer(storeWithTx)

		if err := utxoStoreWithTx.SetBlockHeight(ctx, block.GetHeight()); err != nil {
			if errors.Is(err, utxostore.ErrIsPreviousBlockHeight) {
				return ErrBlockAlreadyStored
			}

			return err
		}

		for _, tx := range block.GetTransactions() {

			err = utxoStoreWithTx.AddTransactionOutputs(
				ctx,
				tx.GetID(),
				getTransactionsOutputsForStore(tx),
			)
			if err != nil {
				return fmt.Errorf("failed to store utxo: %w", err)
			}

			err = u.spendOutputs(ctx, utxoStoreWithTx, tx)
			if err != nil {
				return fmt.Errorf("failed to spend outputs: %w", err)
			}
		}

		return nil
	})
}

func (s *Service) spendOutputs(
	ctx context.Context,
	utxoStore *utxostore.Store,
	tx *blockchain.Transaction,
) error {
	for _, input := range tx.GetInputs() {
		if input.SpendingOutput != nil {
			spentOutput, err := utxoStore.SpendOutput(ctx, input.SpendingOutput.TxID, input.SpendingOutput.VOut)
			if err != nil {
				return fmt.Errorf("failed to spend utxo: %w", err)
			}
			_ = spentOutput
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

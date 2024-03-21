package utxoservice

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
	"github.com/syndtr/goleveldb/leveldb"
)

type UTXOStore interface {
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	GetOutputsByTxID(_ context.Context, txID string) ([]*utxostore.TransactionOutput, error)
	SpendOutput(_ context.Context, txID string, idx int) error
	WithStorer(storer keyvaluestore.Store) *utxostore.Store
}

type ServiceOptions struct {
	Logger *zerolog.Logger
}

type Service struct {
	s UTXOStore

	txManager    *txmanager.TransactionManager[*leveldb.Transaction]
	ldbUTXOStore keyvaluestore.StoreWithTxManager[*leveldb.Transaction]

	// logger is the logger used by the service.
	logger *zerolog.Logger
}

func New(
	s UTXOStore,

	txManager *txmanager.TransactionManager[*leveldb.Transaction],
	utxoKVStore keyvaluestore.StoreWithTxManager[*leveldb.Transaction],

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

func (u *Service) AddFromBlock(ctx context.Context, block *blockchain.Block) error {
	for _, tx := range block.GetTransactions() {

		outputs := make([]*utxostore.TransactionOutput, len(tx.GetOutputs()))

		for i, output := range tx.GetOutputs() {
			outputs[i] = &utxostore.TransactionOutput{
				ScriptBytes: output.ScriptPubKey.HEX,
				Addresses:   getOutputAdresses(output),
				Amount:      output.Value.BigFloat,
			}
		}

		err := u.txManager.Do(func(ctx context.Context, ldbTx txmanager.Transaction[*leveldb.Transaction]) error {
			storeWithTx, err := u.ldbUTXOStore.WithTx(ldbTx)
			if err != nil {
				return err
			}

			utxoStoreWithTx := u.s.WithStorer(storeWithTx)

			err = utxoStoreWithTx.AddTransactionOutputs(ctx, tx.GetID(), outputs)
			if err != nil {
				return fmt.Errorf("failed to store utxo: %w", err)
			}

			for _, input := range tx.GetInputs() {
				if input.SpendingOutput != nil {
					err := utxoStoreWithTx.SpendOutput(ctx, input.SpendingOutput.TxID, input.SpendingOutput.VOut)
					if err != nil {
						return fmt.Errorf("failed to spend utxo: %w", err)
					}
				}
			}

			return fmt.Errorf("rollback it, please")
		})

		if err != nil {
			return fmt.Errorf("failed to store outputs: %w", err)
		}
	}

	return nil
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

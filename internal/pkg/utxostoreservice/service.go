package utxostoreservice

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/unspentoutputs/store"
	"github.com/rs/zerolog"
	"github.com/syndtr/goleveldb/leveldb"
)

type UTXOStore interface {
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*store.TransactionOutput) error
	GetOutputsByTxID(_ context.Context, txID string) ([]*store.TransactionOutput, error)
	SpendOutput(_ context.Context, txID string, idx int) error
	WithStore(s store.KeyValueStore) *store.UnspentOutputsStore
}

type UTXOServiceOptions struct {
	Logger *zerolog.Logger
}

type UTXOStoreService struct {
	s UTXOStore

	txManager    *txmanager.TxManager[*leveldb.Transaction]
	ldbUTXOStore keyvaluestore.KeyValueStore[*leveldb.Transaction]

	// logger is the logger used by the service.
	logger *zerolog.Logger
}

func NewUTXOStoreService(
	s UTXOStore,

	txManager *txmanager.TxManager[*leveldb.Transaction],
	utxoKVStore keyvaluestore.KeyValueStore[*leveldb.Transaction],

	options *UTXOServiceOptions,
) *UTXOStoreService {
	defaultOptions := &UTXOServiceOptions{
		Logger: zerolog.DefaultContextLogger,
	}

	if options != nil {
		if options.Logger != nil {
			defaultOptions.Logger = options.Logger
		}
	}

	return &UTXOStoreService{
		s:            s,
		ldbUTXOStore: utxoKVStore,
		logger:       defaultOptions.Logger,
		txManager:    txManager,
	}
}

func (u *UTXOStoreService) AddFromBlock(ctx context.Context, block *blockchain.Block) error {
	for _, tx := range block.GetTransactions() {

		outputs := make([]*store.TransactionOutput, len(tx.GetOutputs()))

		for i, output := range tx.GetOutputs() {
			outputs[i] = &store.TransactionOutput{
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

			utxoStoreWithTx := u.s.WithStore(storeWithTx)

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

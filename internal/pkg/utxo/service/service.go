package utxoservice

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
)

type UTXOStoreMethods interface {
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	GetOutputsByTxID(_ context.Context, txID string) ([]*utxostore.TransactionOutput, error)
	SpendOutput(_ context.Context, txID string, idx int) ([]string, *utxostore.TransactionOutput, error)
	GetUnspentOutputsByAddress(_ context.Context, address string) ([]*utxostore.UTXOEntry, error)
	GetBlockHeight(_ context.Context) (int64, error)
	GetBlockHash(_ context.Context) (string, error)
	SetBlockHeight(_ context.Context, height int64) error
	SetBlockHash(_ context.Context, hash string) error
	SpendAllOutputs(_ context.Context, txID string) error
	UpgradeTransactionOutputs(_ context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	RemoveAddressTxIDs(_ context.Context, address string, txIDs []string) error
}

type UTXOStore[T any, O UTXOStoreMethods] interface {
	UTXOStoreMethods

	WithTx(t txmanager.Transaction[T]) (O, error)
}

type ServiceOptions struct {
	Logger *zerolog.Logger
}

type Service[T any, UTS UTXOStoreMethods] struct {
	s UTXOStore[T, UTS]

	txManager *txmanager.TransactionManager[T]

	btcConfig BitcoinConfig

	// logger is the logger used by the service.
	logger *zerolog.Logger
}

type BitcoinConfig interface {
	GetDecimals() int
	GetParams() *chaincfg.Params
}

func New[T any, UTS UTXOStoreMethods](
	s UTXOStore[T, UTS],
	txManager *txmanager.TransactionManager[T],

	bitcoinConfig BitcoinConfig,
	options *ServiceOptions,
) *Service[T, UTS] {
	defaultOptions := &ServiceOptions{
		Logger: zerolog.DefaultContextLogger,
	}

	if options != nil {
		if options.Logger != nil {
			defaultOptions.Logger = options.Logger
		}
	}

	return &Service[T, UTS]{
		s:         s,
		logger:    defaultOptions.Logger,
		txManager: txManager,
		btcConfig: bitcoinConfig,
	}
}

func (u *Service[T, UTS]) GetBlockHash(ctx context.Context) (string, error) {
	hash, err := u.s.GetBlockHash(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get block hash from store: %w", err)
	}

	return hash, nil
}

func (u *Service[Ti, UTS]) GetBlockHeight(ctx context.Context) (int64, error) {
	height, err := u.s.GetBlockHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get block height from store: %w", err)
	}

	return height, nil
}

func (u *Service[T, _]) GetUTXOByBase58Address(ctx context.Context, address string) ([]*utxostore.UTXOEntry, error) {
	btcAddr, err := btcutil.DecodeAddress(address, u.btcConfig.GetParams())
	if err != nil {
		return nil, ErrInvalidBase58Address
	}

	hexAddr := hex.EncodeToString(btcAddr.ScriptAddress())
	u.logger.Debug().Str("addr", hexAddr).Msg("getting utxo by this address")

	outputs, err := u.s.GetUnspentOutputsByAddress(ctx, hex.EncodeToString(btcAddr.ScriptAddress()))
	if err != nil {
		u.logger.Error().
			Err(err).
			Str("addr", hexAddr).
			Msg("failed to get unspent outputs by address")

		return nil, fmt.Errorf("failed to get UTXO by address: %w", err)
	}

	return outputs, nil
}

func (u *Service[T, _]) AddFromBlock(ctx context.Context, block *blockchain.Block) error {
	return u.txManager.Do(nil, func(ctx context.Context, tx txmanager.Transaction[T]) error {
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

		utxoStoreWithTx, err := u.s.WithTx(tx)
		if err != nil {
			return err
		}

		utxoStoreWithTx.GetBlockHeight(context.Background())

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

			dereferencedAddresses, err := u.spendOutputs(spendingOutputs, tx)
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

func (s *Service[T, _]) getSpeningOutputs(
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
func (s *Service[T, _]) spendOutputs(
	storedOutputs map[string][]*utxostore.TransactionOutput,
	tx *blockchain.Transaction,
) (map[string]string, error) {

	// address => dereferenced tx id
	dereferencedAddresses := map[string]string{}

	for _, input := range tx.GetInputs() {
		if input.SpendingOutput != nil {

			vout := input.SpendingOutput.VOut

			spendingOutputs, ok := storedOutputs[input.SpendingOutput.TxID]
			if !ok {
				s.logger.Error().Int("vout", vout).Str("txID", input.SpendingOutput.TxID).Msg("failed to spend output, not found outputs set")

				return nil, utxostore.ErrNotFound
			}

			if len(spendingOutputs) <= vout {
				s.logger.Error().Int("vout", vout).Int("spendingCount", len(spendingOutputs)).Str("txID", input.SpendingOutput.TxID).Msg("failed to spend output, not found")

				return nil, utxostore.ErrNotFound
			}

			if spendingOutputs[vout] == nil {
				s.logger.Error().Int("vout", vout).Str("txID", input.SpendingOutput.TxID).Msg("failed to spend output, already spent")

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

			convertedOutput := &utxostore.TransactionOutput{}

			convertedOutput.SetScriptBytes(scriptBytes)
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

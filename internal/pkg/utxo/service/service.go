package utxoservice

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxospending"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
)

type UTXOStoreMethods interface {
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	GetOutputsByTxID(ctx context.Context, txID string) ([]*utxostore.TransactionOutput, error)
	SpendOutput(ctx context.Context, txID string, idx int) ([]string, *utxostore.TransactionOutput, error)
	GetUnspentOutputsByAddress(ctx context.Context, address string) ([]*utxostore.UTXOEntry, error)
	GetBlockHeight(ctx context.Context) (int64, error)
	GetBlockHash(ctx context.Context) (string, error)
	SetBlockHeight(ctx context.Context, height int64) error
	SetBlockHash(ctx context.Context, hash string) error
	SpendAllOutputs(ctx context.Context, txID string) error
	SetTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	RemoveAddressTxIDs(ctx context.Context, address string, txIDs []string) error
	SpendOutputsWithNewBlockInfo(
		ctx context.Context,
		newBlockHash string,
		newBlockHeight int64,
		newTxOutputs map[string][]*utxostore.TransactionOutput,
		dereferencedAddresses map[string][]string,
		newAddressReferences map[string][]string,
	) error
}

type UTXOSpender interface {
	NewCheckpointFromNextBlock(ctx context.Context, newBlock *blockchain.Block) (*utxospending.UTXOSpendingCheckpoint, error)
}

type ServiceOptions struct {
	Logger *zerolog.Logger
}

type UTXOService struct {
	s UTXOStoreMethods

	btcConfig BitcoinConfig

	// logger is the logger used by the service.
	logger *zerolog.Logger

	utxoSpender UTXOSpender
}

type BitcoinConfig interface {
	GetDecimals() int
	GetParams() *chaincfg.Params
}

func NewRedisUTXOService(
	s UTXOStoreMethods,
	bitcoinConfig BitcoinConfig,
	options *ServiceOptions,
	utxoSpender UTXOSpender,
) *UTXOService {
	defaultOptions := &ServiceOptions{
		Logger: zerolog.DefaultContextLogger,
	}

	if options != nil {
		if options.Logger != nil {
			defaultOptions.Logger = options.Logger
		}
	}

	return &UTXOService{
		s:           s,
		logger:      defaultOptions.Logger,
		btcConfig:   bitcoinConfig,
		utxoSpender: utxoSpender,
	}
}

func (u *UTXOService) GetBlockHash(ctx context.Context) (string, error) {
	hash, err := u.s.GetBlockHash(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get block hash from store: %w", err)
	}

	return hash, nil
}

func (u *UTXOService) GetBlockHeight(ctx context.Context) (int64, error) {
	height, err := u.s.GetBlockHeight(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get block height from store: %w", err)
	}

	return height, nil
}

func (u *UTXOService) GetUTXOByBase58Address(ctx context.Context, address string) ([]*utxostore.UTXOEntry, error) {
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

func (u *UTXOService) AddFromBlock(ctx context.Context, block *blockchain.Block) error {
	spendingCheckpoint, err := u.utxoSpender.NewCheckpointFromNextBlock(ctx, block)
	if err != nil {
		if errors.Is(err, utxospending.ErrNoDiff) {
			return ErrBlockAlreadyStored
		}

		return fmt.Errorf("failed to create next spending checkpoint: %w", err)
	}

	if err := u.s.SpendOutputsWithNewBlockInfo(
		ctx,
		spendingCheckpoint.GetNextBlockHash(),
		spendingCheckpoint.GetNewBlockHeight(),
		spendingCheckpoint.GetNewTransactionsOutputs(),
		spendingCheckpoint.GetDereferencedAddressesTxs(),
		spendingCheckpoint.GetNewAddreessReferences(),
	); err != nil {
		return fmt.Errorf("fialed to store new block outputs: %w", err)
	}

	return nil
}

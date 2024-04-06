package chainstatemigration

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bigjson"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/utxo"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

type ChainstateDB interface {
	NewUTXOIterator() *chainstate.UTXOIterator
	GetDeobfuscator() *chainstate.ChainstateDeobfuscator
	GetBlockHash(ctx context.Context) ([]byte, error)
}

type UTXOStore interface {
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	SetBlockHeight(ctx context.Context, blockHeight int64) error
	SetBlockHash(ctx context.Context, blockHash string) error
}

type BitcoinConfig interface {
	GetDecimals() int
	GetParams() *chaincfg.Params
}

type Migrator struct {
	// The chainstate from which to migrate the UTXOs.
	cdb ChainstateDB

	// The UTXO store to migrate the UTXOs to.
	utxoStore UTXOStore

	// Configration of the bitcoin
	bitcoinConfig BitcoinConfig

	// The block height of the chainstate. This is used to set the block height in the UTXO store.
	chainstateBlockHeight int64

	// Logger
	logger *zerolog.Logger
}

func NewMigrator(
	logger *zerolog.Logger,

	cdb ChainstateDB,
	utxoStore UTXOStore,
	bitcoinConfig BitcoinConfig,

	chainstateBlockHeight int64,
) *Migrator {
	return &Migrator{
		logger: logger,

		cdb:           cdb,
		utxoStore:     utxoStore,
		bitcoinConfig: bitcoinConfig,

		chainstateBlockHeight: chainstateBlockHeight,
	}
}

// Migrate migrates the UTXO from the chainstate database to the UTXO store.
func (m *Migrator) Migrate(ctx context.Context) error {
	m.logger.Info().Msg("migrating UTXOs from chainstate to UTXO store")

	utxoIterator := m.cdb.NewUTXOIterator()

	utxoByTxID := []*utxo.TxOut{}
	currentTxID := ""

	for {
		currentUTXO, err := utxoIterator.Next(ctx)
		if err != nil {
			if errors.Is(err, chainstate.ErrNoKeysMore) {
				break
			}

			return fmt.Errorf("failed to iterate UTXOs: %w", err)
		}

		if currentTxID != currentUTXO.GetTxID() {
			if len(utxoByTxID) > 0 {
				m.logger.Debug().
					Str("txid", currentTxID).
					Str("txOutCount", fmt.Sprintf("%d", len(utxoByTxID))).
					Msg("migrating UTXO")

				// migrate utxo grouped by tx id
				outputs := convertUTXOlistToTransactionOutputList(m.bitcoinConfig, utxoByTxID)
				if err := m.utxoStore.AddTransactionOutputs(ctx, currentTxID, outputs); err != nil {
					return fmt.Errorf("failed to add transaction outputs: %w", err)
				}
			}

			m.logger.Debug().
				Str("currentTxID", currentTxID).
				Str("newTxID", currentUTXO.GetTxID()).
				Msg("new txid found")

			currentTxID = currentUTXO.GetTxID()
			utxoByTxID = []*utxo.TxOut{}
		}

		utxoByTxID = append(utxoByTxID, currentUTXO)
	}

	m.logger.Info().Msg("migrating block hash and height")
	blockHash, err := m.cdb.GetBlockHash(ctx)
	if err != nil {
		return fmt.Errorf("failed to get block hash: %w", err)
	}

	m.logger.Debug().
		Str("blockHash", hex.EncodeToString(blockHash)).
		Int64("blockHeight", m.chainstateBlockHeight).
		Msg("migrate block hash and height")

	err = m.utxoStore.SetBlockHash(ctx, hex.EncodeToString(blockHash))
	if err != nil {
		return fmt.Errorf("failed to set block hash: %w", err)
	}

	err = m.utxoStore.SetBlockHeight(ctx, m.chainstateBlockHeight)
	if err != nil {
		return fmt.Errorf("failed to set block height: %w", err)
	}

	m.logger.Info().Msg("migrated UTXOs from chainstate to UTXO store")
	return nil
}

func convertUTXOlistToTransactionOutputList(bitcoinConfig BitcoinConfig, utxos []*utxo.TxOut) []*utxostore.TransactionOutput {
	outputs := make([]*utxostore.TransactionOutput, 0, len(utxos))
	decimals := decimal.NewFromInt(int64(bitcoinConfig.GetDecimals()))

	for _, utxo := range utxos {

		addresses := []string{}
		amountBigF64 := decimal.NewFromInt(utxo.GetCoin().GetOut().Value).Div(decimals).BigFloat()

		if script, err := txscript.ParsePkScript(utxo.GetCoin().GetOut().PkScript); err == nil {
			address, err := script.Address(bitcoinConfig.GetParams())
			if err == nil {
				// address is not nil
				// do something with address
				addresses = append(addresses, address.EncodeAddress())
			}
		}

		outputs = append(outputs, &utxostore.TransactionOutput{
			ScriptBytes: hex.EncodeToString(utxo.GetCoin().GetOut().PkScript),
			Amount:      *bigjson.NewBigFloat(*amountBigF64),
			Addresses:   addresses,
		})
	}

	return outputs
}

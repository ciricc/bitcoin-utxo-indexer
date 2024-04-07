package chainstatemigration

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

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
	ApproximateSize() (int64, error)
}

type UTXOStore interface {
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	SetBlockHeight(ctx context.Context, blockHeight int64) error
	SetBlockHash(ctx context.Context, blockHash string) error
	Flush(ctx context.Context) error
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

	m.logger.Info().Msg("flushing current UTXO store")

	if err := m.utxoStore.Flush(ctx); err != nil {
		return fmt.Errorf("failed to flush store: %w", err)
	}

	countOfKeys, err := m.cdb.ApproximateSize()
	if err != nil {
		return fmt.Errorf("failed to get chainstate approximate size: %w", err)
	}

	m.logger.Debug().Int64("keys", countOfKeys).Msg("chainstate keys count")

	utxoIterator := m.cdb.NewUTXOIterator()

	utxoByTxID := []*utxo.TxOut{}
	currentTxID := ""

	var keyI int64 = 0

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			m.logger.Info().Float64("progress", percentage(keyI, countOfKeys)).Msg("migrating from chainstate")
		}
	}()

	for {
		currentUTXO, err := utxoIterator.Next(ctx)
		if err != nil {
			if errors.Is(err, chainstate.ErrNoKeysMore) {
				break
			}

			return fmt.Errorf("failed to iterate UTXOs: %w", err)
		}

		keyI++

		if currentTxID != currentUTXO.GetTxID() {
			if len(utxoByTxID) > 0 {
				// migrate utxo grouped by tx id
				outputs := convertUTXOlistToTransactionOutputList(m.bitcoinConfig, utxoByTxID)
				if err := m.utxoStore.AddTransactionOutputs(ctx, currentTxID, outputs); err != nil {
					return fmt.Errorf("failed to add transaction outputs: %w", err)
				}
			}

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
	decimalsInt := bitcoinConfig.GetDecimals()

	decimals := decimal.New(1, int32(decimalsInt))

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
			ScriptBytes: utxo.GetCoin().GetOut().PkScript,
			Amount:      *bigjson.NewBigFloat(*amountBigF64),
		})
	}

	return outputs
}

func percentage[T int | int64](a T, b T) float64 {
	if b == 0 {
		return 0
	}

	f := int64(float64(a) / float64(b) * 100_00)

	return float64(f) / 100
}

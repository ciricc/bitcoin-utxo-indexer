package chainstatemigration

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/utxo"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
)

type ChainstateDB interface {
	NewUTXOIterator() *chainstate.UTXOIterator
	GetDeobfuscator() *chainstate.ChainstateDeobfuscator
	GetBlockHash(ctx context.Context) ([]byte, error)
	ApproximateSize() (int64, error)
}

type UTXOStoreMethods interface {
	AreExiststsOutputs(ctx context.Context, txID string) (bool, error)
	AddTransactionOutputs(ctx context.Context, txID string, outputs []*utxostore.TransactionOutput) error
	SetBlockHeight(ctx context.Context, blockHeight int64) error
	SetBlockHash(ctx context.Context, blockHash string) error
	Flush(ctx context.Context) error
}

type UTXOStore[T any, UTS UTXOStoreMethods] interface {
	UTXOStoreMethods

	WithTx(txmanager.Transaction[T]) (UTS, error)
}

type BitcoinConfig interface {
	GetDecimals() int
	GetParams() *chaincfg.Params
}

type txOutputsEntry struct {
	txID    string
	outputs []*utxostore.TransactionOutput
}

type Migrator[T any, UTS UTXOStoreMethods] struct {
	// The chainstate from which to migrate the UTXOs.
	cdb ChainstateDB

	// The UTXO store to migrate the UTXOs to.
	utxoStore UTXOStore[T, UTS]

	// It needed to store the outputs in the batch
	utxoStoreTxManager *txmanager.TransactionManager[T]

	// The block height of the chainstate. This is used to set the block height in the UTXO store.
	chainstateBlockHeight int64

	// The number of transactions will be stored in one batch
	batchSize int

	// Logger
	logger *zerolog.Logger

	// When the migration process fails, it tries to recover and continue migration after
	// It iterates through the current chainstate and getting the UTXO be each txID until getting "not found"
	//
	// Which means that in the chainstate we got the transaction no existing in the database
	// So, we need to add the outputs started from this transaction
	//
	// After we found the end of the migrated transactions, we make this flag to "true"
	// To stop checking the existing txs
	restoredLastSavedTxID bool
}

func NewMigrator[T any, UTS UTXOStoreMethods](
	logger *zerolog.Logger,

	cdb ChainstateDB,

	utxoStore UTXOStore[T, UTS],
	utxoStoreTxManager *txmanager.TransactionManager[T],

	batchSize int,
	chainstateBlockHeight int64,
) *Migrator[T, UTS] {
	return &Migrator[T, UTS]{
		logger: logger,

		cdb:       cdb,
		utxoStore: utxoStore,

		chainstateBlockHeight: chainstateBlockHeight,
		utxoStoreTxManager:    utxoStoreTxManager,

		batchSize:             batchSize,
		restoredLastSavedTxID: false,
	}
}

// Migrate migrates the UTXO from the chainstate database to the UTXO store.
func (m *Migrator[T, _]) Migrate(ctx context.Context) error {
	m.logger.Info().Msg("migrating UTXOs from chainstate to UTXO store")

	m.logger.Info().Msg("flushing current UTXO store")

	// if err := m.utxoStore.Flush(ctx); err != nil {
	// 	return fmt.Errorf("failed to flush store: %w", err)
	// }

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

	utxoBatch := make([]*txOutputsEntry, 0, m.batchSize)

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
				outputs := ConvertUTXOlistToTransactionOutputList(utxoByTxID)
				utxoBatch = append(utxoBatch, &txOutputsEntry{
					txID:    currentTxID,
					outputs: outputs,
				})
			}

			currentTxID = currentUTXO.GetTxID()
			utxoByTxID = []*utxo.TxOut{}
		}

		utxoIdx := int(currentUTXO.Index())

		utxoByTxID, err = PushElementToPlace(utxoByTxID, currentUTXO, utxoIdx)
		if err != nil {
			return fmt.Errorf("failed to push utxo (like panic): %w", err)
		}

		if len(utxoBatch) == m.batchSize {
			for {
				if err := m.storeUTXOBatch(ctx, utxoBatch); err != nil {
					m.logger.Error().Err(err).Msg("failed to stora batch")
					time.Sleep(5 * time.Second)
					continue
				}

				break
			}

			utxoBatch = make([]*txOutputsEntry, 0, m.batchSize)
		}
	}

	if len(utxoBatch) > 0 {
		if err := m.storeUTXOBatch(ctx, utxoBatch); err != nil {
			return fmt.Errorf("failed to store utxo batch: %w", err)
		}

		utxoBatch = nil
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

func (m *Migrator[T, _]) storeUTXOBatch(
	ctx context.Context,
	batch []*txOutputsEntry,
) error {

	if len(batch) == 0 {
		m.logger.Warn().Msg("the batch is empty")

		return nil
	}

	if !m.restoredLastSavedTxID {
		m.logger.Debug().Msg("Trying to restore migration process...")

		allFound := true

		for _, txEntry := range batch {
			found, err := m.utxoStore.AreExiststsOutputs(ctx, txEntry.txID)
			if err != nil {
				return fmt.Errorf("fialed to check outputs: %w", err)
			}

			allFound = allFound && found

			if !found {
				m.logger.Debug().Str("txID", txEntry.txID).Msg("not found tx entry")

				m.restoredLastSavedTxID = true
				break
			}
		}

		if allFound {
			m.logger.Debug().
				Str("firstTxID", batch[0].txID).
				Str("lastTxID", batch[len(batch)-1].txID).
				Msg("found all tx entries")

			// skip set operations
			return nil
		}
	}

	return m.updateUTXObatch(ctx, batch)
}

func (m *Migrator[T, UTS]) updateUTXObatch(
	ctx context.Context,
	batch []*txOutputsEntry,
) error {
	return m.utxoStoreTxManager.Do(nil, func(ctx context.Context, tx txmanager.Transaction[T]) error {
		utxoStoreWithTx, err := m.utxoStore.WithTx(tx)
		if err != nil {
			return err
		}

		for _, txEntry := range batch {
			if err := utxoStoreWithTx.AddTransactionOutputs(ctx, txEntry.txID, txEntry.outputs); err != nil {
				return fmt.Errorf("failed to add transaction outputs: %w", err)
			}
		}

		return nil
	})
}

func ConvertUTXOlistToTransactionOutputList(utxos []*utxo.TxOut) []*utxostore.TransactionOutput {
	outputs := make([]*utxostore.TransactionOutput, 0, len(utxos))

	for _, utxo := range utxos {
		// spent
		if utxo == nil {
			outputs = append(outputs, nil)
			continue
		}

		convertedOutput := &utxostore.TransactionOutput{}

		convertedOutput.SetScriptBytes(utxo.GetCoin().GetOut().PkScript)
		convertedOutput.SetAmount(uint64(utxo.GetCoin().GetOut().Value))

		outputs = append(outputs, convertedOutput)
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

var (
	ErrNoNegativePlaces = errors.New("no negative places")
)

func PushElementToPlace[T any](list []T, element T, placeIdx int) ([]T, error) {
	if placeIdx < 0 {
		return nil, ErrNoNegativePlaces
	}

	if len(list) > placeIdx {
		list[placeIdx] = element

		return list, nil
	} else if len(list) == placeIdx {
		return append(list, element), nil
	} else {
		newList := make([]T, placeIdx+1)
		newList[placeIdx] = element

		copy(newList[0:placeIdx], list)

		return newList, nil
	}
}

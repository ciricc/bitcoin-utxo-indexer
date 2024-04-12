package chainstatemigration

import (
	"context"
	"sync"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/rs/zerolog"
)

type MigrationFixer[T any, UTS UTXOStoreMethods] struct {
	migrator   *Migrator[T, UTS]
	patchTxsCh chan *txOutputsEntry

	logger *zerolog.Logger
}

func NewMigrationFixer[T any, UTS UTXOStoreMethods](logger *zerolog.Logger, migration *Migrator[T, UTS]) *MigrationFixer[T, UTS] {
	return &MigrationFixer[T, UTS]{
		patchTxsCh: make(chan *txOutputsEntry, 100000),
		logger:     logger,
		migrator:   migration,
	}
}

func (m *MigrationFixer[T, UTS]) PushTxToPatch(txID string, outputs []*utxostore.TransactionOutput) {
	m.patchTxsCh <- &txOutputsEntry{
		txID:    txID,
		outputs: outputs,
	}
}

func (m *MigrationFixer[T, UTS]) Run(ctx context.Context) <-chan struct{} {
	quitCh := make(chan struct{}, 1)

	go m.runTxsUpdater(ctx, quitCh)

	return quitCh
}

func (m *MigrationFixer[T, UTS]) runTxsUpdater(ctx context.Context, quitCh chan<- struct{}) {
	batch := []*txOutputsEntry{}
	batchLock := sync.Mutex{}

	updateBatchTicker := time.NewTicker(10 * time.Second)
	waitLastBatchPatchedCh := make(chan struct{}, 1)

	go func() {
		// getting current batch of the updating tx and patching them in a bulk
		defer func() {
			m.logger.Debug().Msg("trying to signal the last batch updated")

			waitLastBatchPatchedCh <- struct{}{}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-updateBatchTicker.C:
				batchLock.Lock()
				if len(batch) == 0 {
					batchLock.Unlock()

					continue
				}

				// trying to put the new batch into migration
				for {
					err := m.migrator.updateUTXObatch(ctx, batch)
					if err != nil {
						m.logger.Error().Err(err).Int("utxoCount", len(batch)).Msg("failed to store new UTXO patch")
						time.Sleep(5 * time.Second)
						continue
					}

					m.logger.Debug().Int("batchSize", len(batch)).Str("txID", batch[len(batch)-1].txID).Msg("stored the batch")

					batch = make([]*txOutputsEntry, 0)
					break
				}

				batchLock.Unlock()
			}
		}
	}()

	waitLastBatchAppendCh := make(chan struct{}, 1)
	go func() {
		for tx := range m.patchTxsCh {
			batchLock.Lock()
			batch = append(batch, tx)
			batchLock.Unlock()
		}

		waitLastBatchAppendCh <- struct{}{}
		m.logger.Debug().Msg("stoped the loop over updating txs")
	}()

	<-ctx.Done()
	m.logger.Debug().Msg("trying to stop the migration fixes")

	close(m.patchTxsCh)

	<-waitLastBatchAppendCh
	updateBatchTicker.Stop()

	m.logger.Debug().Msg("waiting for last batch updated")

	<-waitLastBatchPatchedCh

	close(quitCh)
	m.logger.Debug().Msg("we are done")
}

package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/app"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/utxo"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/chainstatemigration"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/semaphore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/samber/do"
)

func main() {
	// This script fixes the migration bug:
	// In the utxo store stored only spendable UTXO placed not on their index places
	//
	// So, we need to iterate over chainstate, check the valid of each transaction outputs
	// And if the len of outputs will not the same with the utxo store, we need to update outputs

	chainstateContainer := do.New()

	app.ProvideCommonDeps(chainstateContainer)
	app.ProvideUTXOStoreDeps(chainstateContainer)
	app.ProvideRedisDeps(chainstateContainer)
	app.ProvideChainstateDeps(chainstateContainer)
	app.ProvideBitcoinCoreDeps(chainstateContainer)
	app.ProvideMigratorDeps(chainstateContainer)

	logger, err := do.Invoke[*zerolog.Logger](chainstateContainer)
	if err != nil {
		panic(err)
	}

	migrationFixer, err := do.Invoke[*chainstatemigration.MigrationFixer[redis.Pipeliner, *utxostore.Store[redis.Pipeliner]]](chainstateContainer)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to invoke chainstate migration fixer")
	}

	chainstateDB, err := do.Invoke[*chainstate.DB](chainstateContainer)
	if err != nil {
		logger.Fatal().Err(err).Msg("faieled to invoke chainstate db")
	}

	utxoStore, err := do.Invoke[*utxostore.Store[redis.Pipeliner]](chainstateContainer)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to invoke utxo store")
	}

	ctx, cancel := context.WithCancel(context.Background())

	utxoIerator := chainstateDB.NewUTXOIterator()

	currentTxID := ""
	currentUTXOs := []*utxo.TxOut{}

	// run the fixer to checking the new txs
	quitCh := migrationFixer.Run(ctx)

	wg := sync.WaitGroup{}

	logger.Info().Msg("start fixing migration")

	progressTicker := time.NewTicker(time.Second * 5)
	defer progressTicker.Stop()

	sem := semaphore.New(500000)
	defer sem.Close()

	keyI := 0
	go func() {
		for range progressTicker.C {
			logger.Info().Int("keyI", keyI).Str("txID", currentTxID).Msg("fixing the migration")
		}
	}()

	for {
		currentUTXO, err := utxoIerator.Next(ctx)
		if err != nil {
			if errors.Is(err, chainstate.ErrNoKeysMore) {
				break
			}

			logger.Fatal().Err(err).Msg("failed to iterate next UTXO")

			return
		}

		keyI++
		if currentUTXO.GetTxID() != "78d898a678b475aa464ca77d2250bf1eeb89fe777198d7d5bccf47d15e4e6002" && currentTxID != "78d898a678b475aa464ca77d2250bf1eeb89fe777198d7d5bccf47d15e4e6002" {
			continue
		}

		if currentTxID != currentUTXO.GetTxID() {
			if len(currentUTXOs) > 0 {
				// we got the group of UTXOs here
				//
				// migrate utxo grouped by tx id
				outputs := chainstatemigration.ConvertUTXOlistToTransactionOutputList(currentUTXOs)
				logger.Info().Int("len", len(outputs)).Msg("formed outputs")

				sem.Acquire()
				wg.Add(1)

				// run getting current outputs from the utxo store and checking the validity of them
				go func(txID string, outputs []*utxostore.TransactionOutput) {
					defer func() {
						sem.Release()
						wg.Done()
					}()

					utxoFromStore, err := utxoStore.GetOutputsByTxID(ctx, txID)
					if err != nil {
						logger.Fatal().Str("txID", txID).Err(err).Msg("failed to get outputs by tx id")
					}

					logger.Debug().Str("txID", txID).Any("utxos", utxoFromStore).Any("outputs", outputs).Msg("got utxos from the utxo store")

					if len(utxoFromStore) != len(outputs) {
						//p ush to fixer
						logger.Debug().Str("txID", txID).Msg("push the transaction outputs to the fixer")
						migrationFixer.PushTxToPatch(txID, outputs)
					}
				}(currentTxID, outputs)

			}

			currentTxID = currentUTXO.GetTxID()
			currentUTXOs = []*utxo.TxOut{}
		}

		utxoIdx := int(currentUTXO.Index())

		logger.Debug().Int("idx", utxoIdx).Str("txID", currentTxID).Int("len", len(currentUTXOs)).Msg("pushing utxo to list")

		currentUTXOs, err = chainstatemigration.PushElementToPlace(currentUTXOs, currentUTXO, utxoIdx)

		logger.Debug().Int("idx", utxoIdx).Str("txID", currentTxID).Int("len", len(currentUTXOs)).Msg("pushed utxo to list")

		if err != nil {
			logger.Fatal().Err(err).Msg("failed to push the utxo to the list")
		}
	}

	wg.Wait()

	logger.Debug().Msg("canceling the context")

	cancel()

	<-quitCh
}

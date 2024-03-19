package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/blockchainscanner/scanner"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/di"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/unspentoutputs/store"
	"github.com/rs/zerolog"
	"github.com/samber/do"
)

func main() {
	i := do.New()

	do.Provide(i, di.NewConfig)
	do.Provide(i, di.NewLogger)
	do.Provide(i, di.NewShutdowner)
	do.Provide(i, di.NewRedisClient)
	do.Provide(i, di.NewScannerState)
	do.Provide(i, di.NewBitcoinBlocksIterator)
	do.Provide(i, di.NewBlockchainScanner)
	do.Provide(i, di.NewUTXOStore)
	do.Provide(i, di.NewLevelDBStore)

	logger, err := do.Invoke[*zerolog.Logger](i)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scanner, err := do.Invoke[*scanner.Scanner[*blockchain.Block]](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create scanner")
	}

	utxoStore, err := do.Invoke[*store.UnspentOutputsStore](i)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to invoke UTXO store")
	}

	// outputs, err := utxoStore.GetOutputsByTxID(context.Background(), "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b")
	// if err != nil {
	// 	panic(err)
	// }

	// logger.Info().Any("outputs", outputs).Msg("found outputs")
	// return

	go func() {
		t := time.NewTimer(5 * time.Second)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				countOutputs, err := utxoStore.CountOutputs(ctx)
				if err != nil {
					logger.Error().Err(err).Msg("failed to count UTXO outputs")
					continue
				}

				logger.Info().Int64("totalUTXOCount", countOutputs).Msg("counter UTXO total")
				t.Reset(5 * time.Second)
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		if err := scanner.Start(ctx, func(ctx context.Context, block *blockchain.Block) error {
			logger.Info().Str("hash", block.GetHash().String()).Msg("got new block")

			for _, tx := range block.GetTransactions() {

				outputs := make([]*store.TransactionOutput, len(tx.GetOutputs()))

				for i, output := range tx.GetOutputs() {
					outputs[i] = &store.TransactionOutput{
						ScriptBytes: output.ScriptPubKey.HEX,
						Addresses:   getOutputAdresses(output),
						Amount:      output.Value.BigFloat,
					}
				}

				err := utxoStore.AddTransactionOutputs(ctx, tx.GetID(), outputs)
				if err != nil {
					return fmt.Errorf("failed to store utxo: %w", err)
				}

			}

			logger.Info().Str("hash", block.GetHash().String()).Msg("scanned new block")

			return nil
		}); err != nil {
			logger.Fatal().Err(err).Msg("failed to start scanner")
		}
	}()

	go func() {
		<-ctx.Done()

		if err := i.Shutdown(); err != nil {
			logger.Fatal().Err(err).Msg("failed to shutdown the system")
		}

		os.Exit(0)
	}()

	if err := i.ShutdownOnSIGTERM(); err != nil {
		logger.Fatal().Err(err).Msg("failed to shutdown the service")
	}
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

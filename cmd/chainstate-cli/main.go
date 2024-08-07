package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/app"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
	"github.com/samber/do"
	"github.com/urfave/cli/v2"
)

func main() {

	chainstateContainer := do.New()

	app.ProvideCommonDeps(chainstateContainer)
	app.ProvideChainstateDeps(chainstateContainer)

	chainState, err := do.Invoke[*chainstate.DB](chainstateContainer)
	if err != nil {
		panic(err)
	}

	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:  "getblockhash",
				Usage: "get current block hash from the chainstate",
				Action: func(ctx *cli.Context) error {
					blockHash, err := chainState.GetBlockHash(ctx.Context)
					if err != nil {
						return fmt.Errorf("failed to get: %w", err)
					}

					fmt.Println(hex.EncodeToString(blockHash))

					return nil
				},
			},
			{
				Name:  "utxocount",
				Usage: "Returns count of UTXO in the chainstate",
				Action: func(ctx *cli.Context) error {
					countKeys, err := getChainstateKeysCount(ctx.Context, chainState)
					if err != nil {
						return err
					}

					fmt.Println("Result:", countKeys)

					return nil
				},
			},
			{
				Name:      "gettxouts",
				Usage:     "Returnss the list of UTXO of the transaction",
				Args:      true,
				ArgsUsage: "<tx_id>",
				Action: func(ctx *cli.Context) error {
					txIDHex := ctx.Args().Get(0)

					txID, err := hex.DecodeString(txIDHex)
					if err != nil || len(txID) != 32 {
						return fmt.Errorf("invalid tx id")
					}

					outputs, err := chainState.GetOutputs(ctx.Context, txID)
					if err != nil {
						return err
					}

					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")

					err = enc.Encode(outputs)
					if err != nil {
						return fmt.Errorf("failed to marshal result: %w", err)
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func getChainstateKeysCount(ctx context.Context, chainState *chainstate.DB) (int64, error) {
	var count int64

	iterator := chainState.NewUTXOIterator()
	defer iterator.Release()

	fmt.Println("Counting chainstate UTXOs ...")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			fmt.Println("Count:", count)
		}
	}()

	for {
		_, err := iterator.Next(ctx)
		if err != nil {
			if errors.Is(err, chainstate.ErrNoKeysMore) {
				break
			}

			return 0, fmt.Errorf("failed to get next item: %w", err)
		}

		count++
	}

	return count, nil
}

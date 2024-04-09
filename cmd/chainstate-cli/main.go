package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

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
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// This function runs the chainstate checking process
// First, it iterates over all chainstate keys and find them in the
func runChainstateChecker(chainstateContainer *do.Injector) error {
	return nil
}

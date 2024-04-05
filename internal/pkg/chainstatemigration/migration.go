package chainstatemigration

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
)

type ChainstateDB interface {
	NewUTXOIterator() *chainstate.UTXOIterator
	GetDeobfuscator() *chainstate.ChainstateDeobfuscator
	GetBlockHash(ctx context.Context) ([]byte, error)
}

type Migrator struct {
	cdb ChainstateDB
}

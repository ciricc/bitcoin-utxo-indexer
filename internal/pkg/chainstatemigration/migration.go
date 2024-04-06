package chainstatemigration

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/chainstate"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
)

type ChainstateDB interface {
	NewUTXOIterator() *chainstate.UTXOIterator
	GetDeobfuscator() *chainstate.ChainstateDeobfuscator
	GetBlockHash(ctx context.Context) ([]byte, error)
}

type BlockchainRESTClient interface {
	GetBlockchainInfo(ctx context.Context) (*blockchain.BlockchainInfo, error)
}

type UTXOService interface {
	GetBlockHeight(ctx context.Context) (int64, error)
	GetBlockHash(ctx context.Context) (string, error)
}

type Migrator struct {
	cdb         ChainstateDB
	utxoService UTXOService
}

func NewMigrator(
	cdb ChainstateDB,
	node BlockchainRESTClient,
	utxoService UTXOService,
	migrateToService UTXOService,
) *Migrator {
	return &Migrator{
		cdb:         cdb,
		utxoService: utxoService,
	}
}

// Migrate migrates the UTXO from the chainstate database to the UTXO store.
//
// We migrate the data from chainstate only if the chainstate has more relevant data than the UTXO store.
// The migration process consists of the following steps:
// 1. Get the block height from the UTXO store.
// 2. Get the block height from the chainstate database.
// 3. If the chainstate block height is less than the UTXO store block height, return.
// 4. If the block hash not found in the node, return.
// 5. Create the temporal UTXO store.
// 6. Iterate over the UTXO from the chainstate database.
// 7. For each UTXO, add it to the temporal UTXO store through the UTXO service.
// 8. Replace the UTXO store with the temporal UTXO store.
//
// If the migration fails, the UTXO service will continue to use the old UTXO store.
// If the migration succeeds, the UTXO service will use the new UTXO store.
// If them igration failed, after restart the migration will be retried again with the new temporal UTXO store.
// Previous UTXO store will be deleted.
func (m *Migrator) Migrate(ctx context.Context) error {
	return nil
}

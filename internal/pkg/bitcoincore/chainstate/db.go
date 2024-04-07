package chainstate

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/binaryutils"
)

type DB struct {
	ldb          LevelDB
	deobfuscator *ChainstateDeobfuscator
}

func NewDB(ldb LevelDB) (*DB, error) {
	debofsucator, err := newDeobfuscator(ldb)
	if err != nil {
		return nil, fmt.Errorf("failed to create debofuscator: %w", err)
	}

	return &DB{
		ldb:          ldb,
		deobfuscator: debofsucator,
	}, nil
}

func (d *DB) ApproximateSize() (int64, error) {
	iterator := d.ldb.NewIterator(nil, nil)
	var size int64 = 0
	for iterator.Next() {
		size++
	}

	if err := iterator.Error(); err != nil {
		return 0, fmt.Errorf("iterator error: %w", err)
	}

	return size, nil
}

func (d *DB) NewUTXOIterator() *UTXOIterator {
	ldbIterator := d.ldb.NewIterator(nil, nil)

	return newUTXOIterator(ldbIterator, d.deobfuscator)
}

func (d *DB) GetDeobfuscator() *ChainstateDeobfuscator {
	return d.deobfuscator
}

func (d *DB) GetBlockHash(ctx context.Context) ([]byte, error) {
	blockHash, err := d.ldb.Get([]byte{'B'}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get block hash: %w", err)
	}

	deobfuscatedBlockhash, err := d.deobfuscator.Deobfuscate(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("deobfuscate block hash error: %w", err)
	}

	return binaryutils.ReverseBytesWithCopy(deobfuscatedBlockhash), nil
}

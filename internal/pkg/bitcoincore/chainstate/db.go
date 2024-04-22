package chainstate

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/binaryutils"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/bitcoincorecompression"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/utxo"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
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

func (d *DB) GetOutputs(ctx context.Context, txID []byte) ([]*utxo.TxOut, error) {
	txKey := buildTxIDKey(txID)

	iterator := newUTXOIterator(d.ldb.NewIterator(&util.Range{Start: txKey}, nil), d.deobfuscator)
	defer iterator.Release()

	utxos := make([]*utxo.TxOut, 0)

	for {
		utxo, err := iterator.Next(ctx)
		if err != nil {
			if errors.Is(err, ErrNoKeysMore) {
				break
			}

			return nil, fmt.Errorf("failed to iterate over UTXOs")
		}
		if utxo.GetTxID() == hex.EncodeToString(txID) {
			utxos = append(utxos, utxo)
		} else {
			break
		}
	}

	return utxos, nil
}

func buildTxIDKey(txID []byte) []byte {
	txKey := make([]byte, 0, 33)

	txKey = append(txKey, 'C')

	txID = binaryutils.ReverseBytesWithCopy(txID)

	txKey = append(txKey, txID...)

	return txKey
}

func (d *DB) GetOutput(ctx context.Context, txID []byte, index int) (*utxo.TxOut, error) {
	txKey := buildTxIDKey(txID)

	idxBytes := make([]byte, 12)

	offset := bitcoincorecompression.PutVLQ(idxBytes, uint64(index))
	idxBytes = idxBytes[:offset]

	txKey = append(txKey, idxBytes...)

	utxoValue, err := d.ldb.Get(txKey, nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("failed to get utxo: %w", err)
	}

	deobfuscatedCoin, err := d.deobfuscator.Deobfuscate(ctx, utxoValue)
	if err != nil {
		return nil, fmt.Errorf("failed to deobfuscate: %w", err)
	}

	fullTxOutBytes := buildTxOutBytes(txKey[1:], deobfuscatedCoin)

	txOut := utxo.NewTxOut()

	if err := txOut.Deserialize(bytes.NewReader(fullTxOutBytes)); err != nil {
		return nil, fmt.Errorf("failed to deserialize coin: %w", err)
	}

	return txOut, nil
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

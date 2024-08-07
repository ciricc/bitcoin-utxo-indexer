package chainstate

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/utxo"
	"github.com/syndtr/goleveldb/leveldb/iterator"
)

type UTXOIterator struct {
	iterator     iterator.Iterator
	deobfuscator Deobfuscator
}

func newUTXOIterator(
	iterator iterator.Iterator,
	deobf Deobfuscator,
) *UTXOIterator {
	return &UTXOIterator{
		iterator:     iterator,
		deobfuscator: deobf,
	}
}

func (u *UTXOIterator) Next(ctx context.Context) (*utxo.TxOut, error) {
	if !u.iterator.Next() {
		return nil, ErrNoKeysMore
	}

	outpoint := u.iterator.Key()

	// is not an outpoint
	if len(outpoint) < 34 || !(len(outpoint) >= 1 && outpoint[0] == 0x43) {
		return u.Next(ctx)
	}

	outpoint = outpoint[1:]

	obfuscatedValue := u.iterator.Value()

	deobfuscatedValue, err := u.deobfuscator.Deobfuscate(ctx, obfuscatedValue)
	if err != nil {
		return nil, fmt.Errorf("deobfuscate UTXO error: %w", err)
	}

	fullTxOut := buildTxOutBytes(outpoint, deobfuscatedValue)

	txOut := utxo.NewTxOut()

	if err := txOut.Deserialize(bytes.NewReader(fullTxOut)); err != nil {
		return nil, fmt.Errorf("deserialize UTXO error: %w", err)
	}

	return txOut, nil
}

func (u *UTXOIterator) Release() {
	u.iterator.Release()
}

func buildTxOutBytes(outpoint []byte, coin []byte) []byte {
	fullTxOut := make([]byte, 0, len(outpoint)+len(coin))

	fullTxOut = append(fullTxOut, outpoint...)
	fullTxOut = append(fullTxOut, coin...)

	return fullTxOut
}

// deserializeVLQ deserializes the provided variable-length quantity according
// to the format described above.  It also returns the number of bytes
// deserialized.
func DeserializeVLQ(serialized []byte) (uint64, int) {
	var n uint64
	var size int
	for _, val := range serialized {
		size++
		n = (n << 7) | uint64(val&0x7f)
		if val&0x80 != 0x80 {
			break
		}
		n++
	}

	return n, size
}

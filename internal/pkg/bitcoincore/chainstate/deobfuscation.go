package chainstate

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelDB interface {
	Get(key []byte, ro *opt.ReadOptions) (value []byte, err error)
	NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator
	SizeOf(ranges []util.Range) (leveldb.Sizes, error)
}

type Deobfuscator interface {
	// Deobfuscate must return deobfuscated value
	Deobfuscate(ctx context.Context, value []byte) ([]byte, error)
}

type ChainstateDeobfuscator struct {
	obfuscationKey []byte
}

func newDeobfuscator(ldb LevelDB) (*ChainstateDeobfuscator, error) {
	obfuscationKeyKey, err := hex.DecodeString("0e00")
	if err != nil {
		return nil, fmt.Errorf("decode key prefix error: %w", err)
	}

	obfuscationKeyKey = append(obfuscationKeyKey, []byte("obfuscate_key")...)

	obfuscationKey, err := ldb.Get(obfuscationKeyKey, nil)
	if err != nil && !errors.Is(err, leveldb.ErrNotFound) {
		return nil, fmt.Errorf("failed to get obfuscation key: %w", err)
	}

	if obfuscationKey[0] != 0x8 {
		return nil, fmt.Errorf("invalid obfuscation key format")
	}

	obfuscationKey = obfuscationKey[1:]

	return &ChainstateDeobfuscator{
		obfuscationKey: obfuscationKey,
	}, nil
}

func (cd *ChainstateDeobfuscator) ObfuscationKey() []byte {
	return cd.obfuscationKey
}

func (cd *ChainstateDeobfuscator) Deobfuscate(ctx context.Context, value []byte) ([]byte, error) {
	if cd.obfuscationKey == nil {
		return value, nil
	}

	deobfuscated := deobfuscateValue(cd.obfuscationKey, value)

	return deobfuscated, nil
}

func deobfuscateValue(obfuscationKey, value []byte) []byte {
	deobfuscatedValue := make([]byte, len(value))

	copy(deobfuscatedValue, value)

	for i, c := range value {
		// xor
		deobfuscatedValue[i] = c ^ obfuscationKey[i%len(obfuscationKey)]
	}

	return deobfuscatedValue
}

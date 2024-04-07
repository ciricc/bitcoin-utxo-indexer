package utxostore

import (
	"errors"
	"fmt"
	"strings"
)

type StorageKeyType string

var (
	ErrInvalidStorageKeyFormat = errors.New("invalid storage key format")
)

const (
	addressKeyType       = "addr"
	transactionIDKeyType = "tx"
	blockheightKeyType   = "b"
)

type storageKey struct {
	dbVer  string
	prefix StorageKeyType
	key    string
}

func ver(v, key string) string {
	return fmt.Sprintf("%s:%s", v, key)
}

func newBlockHeightKey(ver string) *storageKey {
	return &storageKey{
		dbVer:  ver,
		prefix: blockheightKeyType,
		key:    "height",
	}
}

func newBlockHashKey(ver string) *storageKey {
	return &storageKey{
		dbVer:  ver,
		prefix: blockheightKeyType,
		key:    "hash",
	}
}

func newAddressUTXOTxIDsSetKey(ver string, address string) *storageKey {
	return &storageKey{
		dbVer:  ver,
		prefix: addressKeyType,
		key:    fmt.Sprintf("%s:set", address),
	}
}

func newAddressUTXOTxIDsKey(ver string, address string, txID string) *storageKey {
	return &storageKey{
		dbVer:  ver,
		prefix: addressKeyType,
		key:    fmt.Sprintf("%s:o:%s", address, txID),
	}
}

func newTransactionIDKey(ver, txID string, isOutputs bool) *storageKey {
	suffix := "outputs"
	if !isOutputs {
		suffix = "inputs"
	}

	return &storageKey{
		dbVer:  ver,
		prefix: transactionIDKeyType,
		key:    fmt.Sprintf("%s:%s", txID, suffix),
	}
}

func StorageKeyFromString(s string) (*storageKey, error) {
	keyColumns := strings.SplitN(s, ":", 3)
	if len(keyColumns) < 2 {
		return nil, ErrInvalidStorageKeyFormat
	}

	return &storageKey{
		dbVer:  keyColumns[0],
		prefix: StorageKeyType(keyColumns[1]),
		key:    keyColumns[1],
	}, nil
}

func (s *storageKey) String() string {
	return fmt.Sprintf("%s:%s:%s", s.dbVer, s.prefix, s.key)
}

func (s *storageKey) TypeOf(t StorageKeyType) bool {
	return s.prefix == t
}

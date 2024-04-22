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
	addressKeyType       = "a"
	transactionIDKeyType = "t"
	blockheightKeyType   = "b"
)

type storageKey struct {
	dbVer  string
	prefix StorageKeyType
	key    string
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
		key:    address,
	}
}

func newTransactionIDKey(ver, txID string) *storageKey {
	return &storageKey{
		dbVer:  ver,
		prefix: transactionIDKeyType,
		key:    txID,
	}
}

func StorageKeyFromString(s string) (*storageKey, error) {
	keyColumns := strings.SplitN(s, ":", 3)
	if len(keyColumns) < 3 {
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

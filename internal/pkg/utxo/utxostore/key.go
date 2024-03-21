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
)

type storageKey struct {
	prefix StorageKeyType
	key    string
}

func newTransactionIDKey(txID string) *storageKey {
	return &storageKey{
		prefix: transactionIDKeyType,
		key:    txID,
	}
}

func StorageKeyFromString(s string) (*storageKey, error) {
	keyColumns := strings.SplitN(s, ":", 2)
	if len(keyColumns) < 2 {
		return nil, ErrInvalidStorageKeyFormat
	}

	return &storageKey{
		prefix: StorageKeyType(keyColumns[0]),
		key:    keyColumns[1],
	}, nil
}

func (s *storageKey) String() string {
	return fmt.Sprintf("%s:%s", s.prefix, s.key)
}

func (s *storageKey) TypeOf(t StorageKeyType) bool {
	return s.prefix == t
}

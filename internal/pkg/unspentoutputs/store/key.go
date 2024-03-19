package store

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
	AddressKeyType       = "addr"
	TransactionIDKeyType = "tx"
)

type StorageKey struct {
	prefix StorageKeyType
	key    string
}

func newTransactionIDKey(txId string) *StorageKey {
	return &StorageKey{
		prefix: TransactionIDKeyType,
		key:    txId,
	}
}

func StorageKeyFromString(s string) (*StorageKey, error) {
	keyColumns := strings.SplitN(s, ":", 2)
	if len(keyColumns) < 2 {
		return nil, ErrInvalidStorageKeyFormat
	}

	return &StorageKey{
		prefix: StorageKeyType(keyColumns[0]),
		key:    keyColumns[1],
	}, nil
}

func (s *StorageKey) String() string {
	return fmt.Sprintf("%s:%s", s.prefix, s.key)
}

func (s *StorageKey) TypeOf(t StorageKeyType) bool {
	return s.prefix == t
}

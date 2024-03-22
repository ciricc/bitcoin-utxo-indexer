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
	blockheightKeyType   = "bh"
)

type storageKey struct {
	prefix StorageKeyType
	key    string
}

func newBlockheightKey() *storageKey {
	return &storageKey{
		prefix: blockheightKeyType,
		key:    "current",
	}
}

func newAddressUTXOTxIDsKey(address string) *storageKey {
	return &storageKey{
		prefix: addressKeyType,
		key:    fmt.Sprintf("%s:o", address),
	}
}

func newTransactionIDKey(txID string, isOutputs bool) *storageKey {
	suffix := "outputs"
	if !isOutputs {
		suffix = "inputs"
	}

	return &storageKey{
		prefix: transactionIDKeyType,
		key:    fmt.Sprintf("%s:%s", txID, suffix),
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

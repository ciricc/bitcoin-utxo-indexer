package state

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
)

type KeyValueScannerState struct {
	// contains filtered or unexported fields
	store keyvaluestore.Store

	defaultStartFromBlockHash string
}

func NewStateWithKeyValueStore(
	defaultStartFromBlockHash string,
	store keyvaluestore.Store,
) *KeyValueScannerState {
	return &KeyValueScannerState{
		store:                     store,
		defaultStartFromBlockHash: defaultStartFromBlockHash,
	}
}

func (s *KeyValueScannerState) UpdateLastScannedBlockHash(ctx context.Context, hash string) error {

	if err := s.store.Set(ctx, "lastScannedBlockHash", hash); err != nil {
		return fmt.Errorf("faild to set lastScannedBlockHash: %w", err)
	}

	return nil
}

func (s *KeyValueScannerState) GetLastScannedBlockHash(ctx context.Context) (string, error) {
	var hash string
	found, err := s.store.Get(ctx, "lastScannedBlockHash", &hash)
	if err != nil {
		return "", fmt.Errorf("faild to get lastScannedBlockHash: %w", err)
	}

	if !found {
		return s.defaultStartFromBlockHash, nil
	}

	return hash, nil
}

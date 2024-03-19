package state

import (
	"context"
	"fmt"

	"github.com/philippgille/gokv"
)

type KVStoreState struct {
	// contains filtered or unexported fields
	store gokv.Store

	defaultStartFromBlockHash string
}

func NewStateWithKeyValueStore(
	defaultStartFromBlockHash string,
	store gokv.Store,
) *KVStoreState {
	return &KVStoreState{
		store:                     store,
		defaultStartFromBlockHash: defaultStartFromBlockHash,
	}
}

func (s *KVStoreState) UpdateLastScannedBlockHash(ctx context.Context, hash string) error {

	if err := s.store.Set("lastScannedBlockHash", hash); err != nil {
		return fmt.Errorf("faild to set lastScannedBlockHash: %w", err)
	}

	return nil
}

func (s *KVStoreState) GetLastScannedBlockHash(ctx context.Context) (string, error) {
	var hash string
	found, err := s.store.Get("lastScannedBlockHash", &hash)
	if err != nil {
		return "", fmt.Errorf("faild to get lastScannedBlockHash: %w", err)
	}

	if !found {
		return s.defaultStartFromBlockHash, nil
	}

	return hash, nil
}

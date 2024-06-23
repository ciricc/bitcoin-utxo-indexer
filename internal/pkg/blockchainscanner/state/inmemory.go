package state

import (
	"context"
)

type InMemoryState struct {
	lastScannedBlockHash string
}

func NewInMemoryState(startFromBlockHash string) *InMemoryState {
	return &InMemoryState{
		lastScannedBlockHash: startFromBlockHash,
	}
}

func (s *InMemoryState) GetLastScannedBlockHash(context.Context) (string, error) {
	return s.lastScannedBlockHash, nil
}

func (s *InMemoryState) UpdateLastScannedBlockHash(_ context.Context, hash string) error {
	s.lastScannedBlockHash = hash

	return nil
}

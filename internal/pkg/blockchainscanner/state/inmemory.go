package state

import (
	"context"
	"fmt"
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

	return fmt.Errorf("something went wrong")
}

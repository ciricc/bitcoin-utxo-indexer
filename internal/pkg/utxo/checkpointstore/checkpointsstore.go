package checkpointstore

import (
	"context"
	"fmt"
	"os"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/vmihailenco/msgpack/v5"
)

type CheckpointStore struct {
	filePath string
}

func NewCheckpointStore(checkpointFilePath string) *CheckpointStore {
	return &CheckpointStore{
		filePath: checkpointFilePath,
	}
}

func (c *CheckpointStore) LastCheckpoint(ctx context.Context) (bool, *utxostore.UTXOSpendingCheckpoint, error) {
	checkpointData, err := os.ReadFile(c.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}

		return true, nil, fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	checkPoint := utxostore.UTXOSpendingCheckpoint{}

	err = msgpack.Unmarshal(checkpointData, &checkPoint)
	if err != nil {
		return true, nil, fmt.Errorf("failed to decode checkpoint data: %w", err)
	}

	return true, &checkPoint, nil
}

func (c *CheckpointStore) SaveCheckpoint(ctx context.Context, checkpoint *utxostore.UTXOSpendingCheckpoint) error {
	checkPointBytes, err := msgpack.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("failed to encode checkpoint data: %w", err)
	}

	err = os.WriteFile(c.filePath, checkPointBytes, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint data: %w", err)
	}

	return nil
}

func (c *CheckpointStore) RemoveLatestCheckpoint(ctx context.Context) error {
	err := os.Remove(c.filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove checkpoint file: %w", err)
	}

	return nil
}

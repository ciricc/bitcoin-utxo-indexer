package migrationmanager

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
)

type Counters interface {
	GetCounter(ctx context.Context, name string) (int64, error)
	IncrCounterBy(ctx context.Context, name string, add int64) (int64, error)
}

type Manager struct {
	c sets.Sets

	versionCounterName string

	migrationID int64
}

func NewManager(name string, counters sets.Sets) (*Manager, error) {
	return &Manager{
		c:                  counters,
		versionCounterName: fmt.Sprintf("migration_versions:%s", name),
		migrationID:        0,
	}, nil
}

func (m *Manager) SetMigrationID(id int64) error {
	if m.migrationID > 0 {
		return ErrMigrationIDAlreadySet
	}

	m.migrationID = id

	return nil
}

func (m *Manager) GetVersion(ctx context.Context) (int64, error) {
	oldestVersion, err := m.getOldestVersion(ctx)
	if err != nil {
		return 0, err
	}

	return oldestVersion + m.migrationID, nil
}

func (m *Manager) DeleteVersion(ctx context.Context, ver int64) error {
	err := m.c.RemoveFromSet(ctx, m.versionCounterName, strconv.FormatInt(ver, 10))
	if err != nil {
		return fmt.Errorf("failed to delete versions: %w", err)
	}

	return nil
}

func (m *Manager) GetVersions(ctx context.Context) ([]int64, error) {
	currentVersions, err := m.c.GetSet(ctx, m.versionCounterName)
	if err != nil {
		return nil, fmt.Errorf("failed to get versions: %w", err)
	}

	if len(currentVersions) == 0 {
		return nil, nil
	}

	currentVersionsInt, err := castVersionsFromStrings(currentVersions)
	if err != nil {
		return nil, fmt.Errorf("failed to cast versions: %w", err)
	}

	return currentVersionsInt, nil
}

func (m *Manager) getOldestVersion(ctx context.Context) (int64, error) {
	currentVersions, err := m.GetVersions(ctx)
	if err != nil {
		return 0, err
	}

	if len(currentVersions) == 0 {
		return 0, nil
	}

	return currentVersions[0], nil
}

func (m *Manager) UpdateVersion(ctx context.Context) (int64, error) {
	if m.migrationID <= 0 {
		return 0, ErrMigrationIDNotSet
	}

	v, err := m.getOldestVersion(ctx)
	if err != nil {
		return 0, err
	}

	newVal := v + m.migrationID

	err = m.c.AddToSet(ctx, m.versionCounterName, strconv.FormatInt(newVal, 10))
	if err != nil {
		return 0, fmt.Errorf("failed to increment version: %w", err)
	}

	return newVal, nil
}

func castVersionsFromStrings(versions []string) ([]int64, error) {
	res := make([]int64, 0, len(versions))
	for _, v := range versions {
		iv, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse version: %w", err)
		}
		res = append(res, iv)
	}

	return res, nil
}

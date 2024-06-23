package migrationmanager

import "errors"

var (
	ErrMigrationIDAlreadySet = errors.New("migration ID already set")
	ErrMigrationIDNotSet     = errors.New("migration ID not set")
	ErrMigrationNotInited    = errors.New("migration not inited")
)

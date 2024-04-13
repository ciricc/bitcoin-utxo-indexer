package config

import (
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/deploy"
	"github.com/go-ozzo/ozzo-validation/is"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type Config struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"env"`

	BlockchainState struct {
		Path string `yaml:"path"`
	} `yaml:"blockchainState"`

	BlockchainParams struct {
		Decimals int `yaml:"decimals"`
	} `yaml:"blockchainParams"`

	ChainstateMigration struct {
		BatchSize int `yaml:"batchSize"`
	} `yaml:"chainstateMigration"`

	BlockchainNode struct {
		RestURL string `yaml:"restURL"`
	} `yaml:"blockchainNode"`

	BlockchainBlocksIterator struct {
		BlockHeadersBufferSize        int   `yaml:"blockHeadersBufferSize"`
		ConcurrentBlocksDownloadLimit int64 `yaml:"concurrentBlocksDownloadLimit"`
	} `yaml:"blockchainBlocksIterator"`

	Scanner struct {
		Enabled bool `yaml:"enabled"`
	}

	UTXO struct {
		Snapshot struct {
			FilePath string `yaml:"filePath"`
		} `yaml:"snapshot"`

		Service struct {
			GRPC struct {
				Address string `yaml:"address"`
			} `yaml:"grpc"`
		} `yaml:"service"`

		Storage struct {
			InMemory struct {
				PersistenceFilePath string `yaml:"persistenceFilePath"`
			} `yaml:"inMemory"`

			LevelDB struct {
				Path string `yaml:"path"`
			} `yaml:"leveldb"`

			Redis struct {
				Host     string `yaml:"host"`
				Username string `yaml:"username"`
				Password string `yaml:"password"`
				DB       int    `yaml:"db"`
			} `yaml:"redis"`
		} `yaml:"storage"`
	} `yaml:"utxo"`

	Uptrace struct {
		DSN string `yaml:"dsn"`
	} `yaml:"uptrace"`
}

func (c Config) Validate() error {

	if err := validation.ValidateStruct(
		&c.BlockchainNode,
		validation.Field(&c.BlockchainNode.RestURL, validation.Required, is.URL),
	); err != nil {
		return fmt.Errorf("failed to validate blockchainNode options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.UTXO.Storage.InMemory,
		validation.Field(&c.UTXO.Storage.InMemory.PersistenceFilePath, validation.Length(0, -1)),
	); err != nil {
		return fmt.Errorf("in-memory configuration validation error: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.UTXO.Storage.LevelDB,
		validation.Field(&c.UTXO.Storage.LevelDB.Path, validation.Length(0, -1)),
	); err != nil {
		return fmt.Errorf("failed to validate utxo storage configuration: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.ChainstateMigration,
		validation.Field(&c.ChainstateMigration.BatchSize, validation.Required, validation.Min(1)),
	); err != nil {
		return fmt.Errorf("failed to validate chainstate migration options options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.UTXO.Storage.Redis,
		validation.Field(&c.UTXO.Storage.Redis.Host, validation.Required, is.DialString),
	); err != nil {
		return fmt.Errorf("failed to validate utxo redis store options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.BlockchainBlocksIterator,
		validation.Field(&c.BlockchainBlocksIterator.BlockHeadersBufferSize, validation.Required, validation.Min(1)),
		validation.Field(&c.BlockchainBlocksIterator.ConcurrentBlocksDownloadLimit, validation.Required, validation.Min(1)),
	); err != nil {
		return fmt.Errorf("failed to validate blockchainBlocksIterator options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.Uptrace,
		validation.Field(&c.Uptrace.DSN, is.URL),
	); err != nil {
		return fmt.Errorf("failed to validate uptrace options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.UTXO.Service.GRPC,
		validation.Field(&c.UTXO.Service.GRPC.Address, validation.Required, is.DialString),
	); err != nil {
		return fmt.Errorf("failed to validate utxo grpc service options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c,
		validation.Field(&c.Name, validation.Required),
		validation.Field(&c.Version, validation.Required, is.Semver),
		validation.Field(&c.Environment, validation.Required, validation.In(deploy.DEV, deploy.PREPROD, deploy.PROD, deploy.STAGE)),
	); err != nil {
		return fmt.Errorf("failed to validate config options: %w", err)
	}

	return nil
}

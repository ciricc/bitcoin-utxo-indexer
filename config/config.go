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

	BlockchainNode struct {
		RestURL string `yaml:"restURL"`
	} `yaml:"blockchainNode"`

	BlockchainBlocksIterator struct {
		BlockHeadersBufferSize        int   `yaml:"blockHeadersBufferSize"`
		ConcurrentBlocksDownloadLimit int64 `yaml:"concurrentBlocksDownloadLimit"`
	} `yaml:"blockchainBlocksIterator"`

	ScannerState struct {
		StartFromBlockHash string `yaml:"startFromBlockHash"`
		RedisStore         struct {
			Host string `yaml:"host"`
		} `yaml:"redisStore"`
	} `yaml:"scannerState"`

	UTXO struct {
		Service struct {
			GRPC struct {
				Address string `yaml:"address"`
			} `yaml:"grpc"`
		} `yaml:"service"`

		Storage struct {
			LevelDB struct {
				Path string `yaml:"path"`
			} `yaml:"leveldb"`
		} `yaml:"storage"`
	} `yaml:"utxo"`
}

func (c Config) Validate() error {

	if err := validation.ValidateStruct(
		&c.BlockchainNode,
		validation.Field(&c.BlockchainNode.RestURL, validation.Required, is.URL),
	); err != nil {
		return fmt.Errorf("failed to validate blockchainNode options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.UTXO.Storage.LevelDB,
		validation.Field(&c.UTXO.Storage.LevelDB.Path, validation.Length(0, -1)),
	); err != nil {
		return fmt.Errorf("failed to validate utxo storage configuration: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.BlockchainBlocksIterator,
		validation.Field(&c.BlockchainBlocksIterator.BlockHeadersBufferSize, validation.Required, validation.Min(1)),
		validation.Field(&c.BlockchainBlocksIterator.ConcurrentBlocksDownloadLimit, validation.Required, validation.Min(1)),
	); err != nil {
		return fmt.Errorf("failed to validate blockchainBlocksIterator options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.ScannerState,
		validation.Field(&c.ScannerState.StartFromBlockHash, validation.Required, is.Hexadecimal),
	); err != nil {
		return fmt.Errorf("failed to validate scannerState options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.UTXO.Service.GRPC,
		validation.Field(&c.UTXO.Service.GRPC.Address, validation.Required, is.DialString),
	); err != nil {
		return fmt.Errorf("failed to validate utxo grpc service options: %w", err)
	}

	if err := validation.ValidateStruct(
		&c.ScannerState.RedisStore,
		validation.Field(&c.ScannerState.RedisStore.Host, validation.Required, is.DialString),
	); err != nil {
		return fmt.Errorf("failed to validate redisStore options: %w", err)
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

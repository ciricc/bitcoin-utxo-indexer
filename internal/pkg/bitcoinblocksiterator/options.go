package bitcoinblocksiterator

import (
	"time"

	"github.com/rs/zerolog"
)

type BitcooinBlocksIteratorOptions struct {
	logger                        *zerolog.Logger
	waitAfterErrorDuration        time.Duration
	downloadHeadersInterval       time.Duration
	concurrentBlocksDownloadLimit int64
	blockHeadersBufferSize        int
}

type BitcoinBlocksIteratorOption func(*BitcooinBlocksIteratorOptions) error

func WithLogger(logger *zerolog.Logger) BitcoinBlocksIteratorOption {
	return func(o *BitcooinBlocksIteratorOptions) error {
		o.logger = logger

		return nil
	}
}

func WithWaitAfterErrorDuration(duration time.Duration) BitcoinBlocksIteratorOption {
	return func(o *BitcooinBlocksIteratorOptions) error {
		o.waitAfterErrorDuration = duration

		return nil
	}
}

func WithDownloadHeadersInterval(duration time.Duration) BitcoinBlocksIteratorOption {
	return func(o *BitcooinBlocksIteratorOptions) error {
		o.downloadHeadersInterval = duration

		return nil
	}
}

func WithConcurrentBlocksDownloadLimit(limit int64) BitcoinBlocksIteratorOption {
	return func(o *BitcooinBlocksIteratorOptions) error {
		if limit <= 0 {
			return ErrInvalidConcurrentBlocksDownloadLimit
		}

		o.concurrentBlocksDownloadLimit = limit

		return nil
	}
}

func WithBlockHeadersBufferSize(size int) BitcoinBlocksIteratorOption {
	return func(o *BitcooinBlocksIteratorOptions) error {
		if size <= 0 {
			return ErrInvalidBlockHeadersBufferSize
		}

		o.blockHeadersBufferSize = size

		return nil
	}
}

func buildOptions(opts ...BitcoinBlocksIteratorOption) (*BitcooinBlocksIteratorOptions, error) {
	options := &BitcooinBlocksIteratorOptions{
		logger:                        zerolog.DefaultContextLogger,
		waitAfterErrorDuration:        time.Second * 5,
		downloadHeadersInterval:       time.Second * 30,
		concurrentBlocksDownloadLimit: 1,
		blockHeadersBufferSize:        1,
	}

	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	return options, nil
}

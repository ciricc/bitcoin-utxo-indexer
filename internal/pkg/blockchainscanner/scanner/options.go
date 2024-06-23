package scanner

import (
	"time"

	"github.com/rs/zerolog"
)

type ScannerOption func(*ScannerOptions) error

type ScannerOptions struct {
	waitAfterErrorDuration time.Duration
	scanInterval           time.Duration

	logger *zerolog.Logger
}

func WithScanInterval(duration time.Duration) ScannerOption {
	return func(so *ScannerOptions) error {
		so.scanInterval = duration

		return nil
	}
}

func WithWaitAfterErrorDuration(duration time.Duration) ScannerOption {
	return func(o *ScannerOptions) error {
		o.waitAfterErrorDuration = duration

		return nil
	}
}

func WithLogger(logger *zerolog.Logger) ScannerOption {
	return func(o *ScannerOptions) error {
		o.logger = logger

		return nil
	}
}

func buildOptions(opts ...ScannerOption) *ScannerOptions {
	options := &ScannerOptions{
		logger:                 zerolog.DefaultContextLogger,
		waitAfterErrorDuration: time.Second * 5,
		scanInterval:           time.Second * 30,
	}

	for _, opt := range opts {
		opt(options)
	}

	return options
}

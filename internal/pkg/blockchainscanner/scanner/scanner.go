package scanner

import (
	"context"
	"fmt"
	"time"
)

type Scanner[B any] struct {
	opts *ScannerOptions
	bi   BlockchainIterator[B]

	state ScannerState
}

type BlockchainIterator[B any] interface {
	// Iterate returns a channel with blocks
	Iterate(context.Context, string) (<-chan B, error)
}

type ScannerState interface {
	// GetLastScannedBlockHash returns the last scanned block hash
	GetLastScannedBlockHash(context.Context) (string, error)

	// UpdateLastScannedBlockHash updates the last scanned block hash
	UpdateLastScannedBlockHash(context.Context, string) error
}

func NewScannerWithState[B any](
	blockchainIterator BlockchainIterator[B],
	state ScannerState,
	opts ...ScannerOption,
) (*Scanner[B], error) {
	options := buildOptions(opts...)

	return &Scanner[B]{
		opts:  options,
		bi:    blockchainIterator,
		state: state,
	}, nil
}

func (s *Scanner[B]) Start(
	ctx context.Context,
	handleBlocks func(ctx context.Context, block B) error,
) error {
	fromBlockHash, err := s.state.GetLastScannedBlockHash(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last scanned block hash: %w", err)
	}

	s.opts.logger.Info().
		Ctx(ctx).
		Str("fromBlockHash", fromBlockHash).
		Msg("start scanning")

	blocks, err := s.bi.Iterate(ctx, fromBlockHash)
	if err != nil {
		return fmt.Errorf("failed to get blockchain items from iterator: %w", err)
	}

	for block := range blocks {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				err := handleBlocks(ctx, block)
				if err != nil {
					s.opts.logger.Error().
						Dur("waitFor", s.opts.waitAfterErrorDuration).
						Err(err).
						Msg("failed to handle new block")

					time.Sleep(s.opts.waitAfterErrorDuration)

					continue
				}
			}
			break
		}
	}

	s.opts.logger.Info().
		Ctx(ctx).
		Msg("scanner stopped")

	return nil
}

package blockchaininfo

import (
	"context"
	"fmt"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

type BlockchainClient interface {
	// GetBlockchainInfo must return the current blockchain info
	GetBlockchainInfo(ctx context.Context) (*blockchain.BlockchainInfo, error)
}

type BitcoinConfig interface {
	GetBlockGenerationInterval() time.Duration
}

type BlockchainInfo struct {
	latestBlockchainInfo *blockchain.BlockchainInfo
	btcConfig            BitcoinConfig
	btcClient            BlockchainClient
	logger               *zerolog.Logger

	cancel context.CancelFunc
}

func New(logger *zerolog.Logger, client BlockchainClient, btcConfig BitcoinConfig) (*BlockchainInfo, error) {
	blockchainInfo, err := client.GetBlockchainInfo(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get blockchain info: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	i := BlockchainInfo{
		latestBlockchainInfo: blockchainInfo,
		btcConfig:            btcConfig,
		btcClient:            client,
		logger:               logger,

		cancel: cancel,
	}

	go i.runSyncing(ctx)

	return &i, nil
}

func (b *BlockchainInfo) runSyncing(ctx context.Context) {
	nextBlockGenerationTime := b.latestBlockchainInfo.GetTime().Add(
		b.btcConfig.GetBlockGenerationInterval(),
	)

	ticker := time.NewTimer(time.Until(nextBlockGenerationTime))
	rl := rate.NewLimiter(rate.Every(b.btcConfig.GetBlockGenerationInterval()), 1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.Wait(ctx)
			for {
				b.logger.Info().
					Int64("currentBlockHeight", b.latestBlockchainInfo.GetBlocksCount()).
					Msg("updating blockchain info")

				blockchainInfo, err := b.btcClient.GetBlockchainInfo(ctx)
				if err != nil {
					b.logger.Error().Err(err).Msg("failed toget blockchain info")
					time.Sleep(time.Second * 5)
					continue
				}

				b.latestBlockchainInfo = blockchainInfo

				nextBlockGenerationTime := b.latestBlockchainInfo.GetTime().Add(
					b.btcConfig.GetBlockGenerationInterval(),
				)

				b.logger.Debug().Dur("generationInterval", b.btcConfig.GetBlockGenerationInterval()).Msg("next block generation info")

				ticker.Reset(time.Until(nextBlockGenerationTime))

				break
			}
		}
	}
}

func (b *BlockchainInfo) Stop() error {
	if b.cancel == nil {
		return fmt.Errorf("already stopped")
	}

	b.cancel()
	b.cancel = nil

	return nil
}

func (b *BlockchainInfo) GetBlocksCount(ctx context.Context) (int64, error) {
	return b.latestBlockchainInfo.GetBlocksCount(), nil
}

func (b *BlockchainInfo) GetHeadersCount(ctx context.Context) (int64, error) {
	return b.latestBlockchainInfo.GetHeadersCount(), nil
}

package bitcoinblocksiterator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/semaphore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/puzpuzpuz/xsync"
)

type BlockchainRESTClient interface {
	GetBlock(ctx context.Context, hash blockchain.Hash) (*blockchain.Block, error)
	GetBlockHash(ctx context.Context, height int64) (*blockchain.Hash, error)
	GetBlockHeader(ctx context.Context, hash blockchain.Hash) (*blockchain.BlockHeader, error)
}

// BitcoinBlocksIterator is an iterator for getting the blocks from the bitcoin blockchain
// It is used for getting the blocks from the blockchain in the right order
type BitcoinBlocksIterator struct {
	opts *BitcooinBlocksIteratorOptions

	restClient BlockchainRESTClient
}

func NewBitcoinBlocksIterator(
	universalRESTClient BlockchainRESTClient,
	opts ...BitcoinBlocksIteratorOption,
) (*BitcoinBlocksIterator, error) {
	options, err := buildOptions(opts...)
	if err != nil {
		return nil, err
	}

	return &BitcoinBlocksIterator{
		opts:       options,
		restClient: universalRESTClient,
	}, nil
}

// Iterate returns a channel with blocks
// It begin downloading the headers and blocks from the blockchain parallel
// startFromBlockHash is a hash of the block from which the download will begin
// You can stop the process by closing the context
func (b *BitcoinBlocksIterator) Iterate(
	ctx context.Context,
	startFromBlockHash string,
) (<-chan *blockchain.Block, error) {
	startFromBlockHashBytes, err := blockchain.NewHashFromHEX(startFromBlockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start from block hash: %w", err)
	}

	blockHeader, err := b.restClient.GetBlockHeader(ctx, startFromBlockHashBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to get start from block header: %w", err)
	}

	downloadBlockHashesCh := make(chan blockchain.Hash, b.opts.blockHeadersBufferSize)

	go b.downloadBlockHeaders(ctx, blockHeader.GetHeight(), downloadBlockHashesCh)

	return b.downloadBlocks(ctx, startFromBlockHashBytes, downloadBlockHashesCh), nil
}

func (s *BitcoinBlocksIterator) downloadBlocks(
	ctx context.Context,
	startedFrom blockchain.Hash,
	blockHashesToDownload <-chan blockchain.Hash,
) <-chan *blockchain.Block {
	s.opts.logger.Info().
		Str("startedFrom", startedFrom.String()).
		Int64("concurrentBlocksDownloadLimit", s.opts.concurrentBlocksDownloadLimit).
		Msg("begin downloading blocks")

	downloadedBlocks := make(chan *blockchain.Block, s.opts.concurrentBlocksDownloadLimit)
	downloadBlocksMx := sync.Mutex{}

	orderingBlocks := xsync.NewMapOf[*blockchain.Block]()

	expectedNextHashToSend := startedFrom

	go func() {
		defer close(downloadedBlocks)
		defer s.opts.logger.Info().Msg("stopkped downloading blocks")

		// The pub/sub for subscribe to the previous block download event
		// It is needed for make from disordered blocks chain to ordered after download of them
		// completed
		// So, the handler will handle whole blockchain in the right order block-by-block
		// withoout download speed reducing

		// Limiting the number of concurrent goroutines to download the blocks
		downloadBlocksSemaphore := semaphore.New(s.opts.concurrentBlocksDownloadLimit)

		for blockHash := range blockHashesToDownload {
			// Requesting a place fo the goroutine
			downloadBlocksSemaphore.Acquire()

			s.opts.logger.Info().
				Str("hash", blockHash.String()).
				Msg("got new header, begin downloading the block")

			go func() {
				// Releasing the place for the next goroutine
				defer downloadBlocksSemaphore.Release()

				var block *blockchain.Block

				for {
					select {
					case <-ctx.Done():
						return
					default:
						blockEntity, err := s.restClient.GetBlock(ctx, blockHash)
						if err != nil {
							s.opts.logger.Error().
								Str("hash", blockHash.String()).
								Dur("waitFor", s.opts.waitAfterErrorDuration).
								Err(err).
								Msg(
									"failed to download block",
								)

							time.Sleep(s.opts.waitAfterErrorDuration)

							continue
						}

						block = blockEntity
					}
					break
				}

				s.opts.logger.Info().
					Str("hash", block.GetHash().String()).
					Str("headerHash", blockHash.String()).
					Str("nextHash", expectedNextHashToSend.String()).
					Int64("size", block.GetSize()).
					Msg("downloaded the block")

				downloadBlocksMx.Lock()
				defer downloadBlocksMx.Unlock()

				orderingBlocks.Store(block.GetHash().String(), block)

				for nextBlock, ok := orderingBlocks.Load(expectedNextHashToSend.String()); ok; nextBlock, ok = orderingBlocks.Load(expectedNextHashToSend.String()) {
					s.opts.logger.Debug().
						Str("hash", nextBlock.GetHash().String()).
						Str("nextHash", nextBlock.GetNextBlockHash().String()).
						Msg("sending new block")
					downloadedBlocks <- nextBlock

					orderingBlocks.Delete(expectedNextHashToSend.String())

					expectedNextHashToSend = nextBlock.GetNextBlockHash()
				}

			}()
		}
	}()

	return downloadedBlocks
}

func (s *BitcoinBlocksIterator) downloadBlockHeaders(
	ctx context.Context,
	fromBlockHeight int64,
	downloadBlockHashes chan<- blockchain.Hash,
) {
	defer close(downloadBlockHashes)

	s.opts.logger.Info().Int64("fromBlockHeight", fromBlockHeight).Int("blockHeadersBufferSize", s.opts.blockHeadersBufferSize).Msg(
		"begin downloading block headers",
	)

	// currentBlockHash := fromBlockHash
	currentBlockHeight := fromBlockHeight

	// First, we get the block headers in order to create a batch of blocks
	// Getting header is faster than getting full block information
	// So, after getting only headers, we can get all blocks parallely
	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.opts.logger.Info().Int64("height", currentBlockHeight).Msg("getting new header")

			downloadedBlockHash, err := s.restClient.GetBlockHash(ctx, currentBlockHeight)
			if err != nil {
				s.opts.logger.Error().Int64("height", currentBlockHeight).Err(err).Msg("failed to get block header")

				time.Sleep(s.opts.waitAfterErrorDuration)

				continue
			}

			currentBlockHeight++
			downloadBlockHashes <- *downloadedBlockHash
		}
	}
}

package bitcoinblocksiterator

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/semaphore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/restclient"
	"github.com/cskr/pubsub"
)

// BitcoinBlocksIterator is an iterator for getting the blocks from the bitcoin blockchain
// It is used for getting the blocks from the blockchain in the right order
type BitcoinBlocksIterator struct {
	opts *BitcooinBlocksIteratorOptions

	restClient *restclient.RESTClient
}

func NewBitcoinBlocksIterator(
	nodeHost *url.URL,
	opts ...BitcoinBlocksIteratorOption,
) (*BitcoinBlocksIterator, error) {
	options, err := buildOptions(opts...)
	if err != nil {
		return nil, err
	}

	restClient, err := restclient.New(nodeHost)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest client: %w", err)
	}

	return &BitcoinBlocksIterator{
		opts:       options,
		restClient: restClient,
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

	headersCh := make(chan *blockchain.BlockHeader, b.opts.blockHeadersBufferSize)

	go b.downloadBlockHeaders(ctx, startFromBlockHashBytes, headersCh)

	return b.downloadBlocks(ctx, startFromBlockHashBytes, headersCh), nil
}

func (s *BitcoinBlocksIterator) downloadBlocks(
	ctx context.Context,
	startedFrom blockchain.Hash,
	headersCh <-chan *blockchain.BlockHeader,
) <-chan *blockchain.Block {
	s.opts.logger.Info().
		Str("startedFrom", startedFrom.String()).
		Int64("concurrentBlocksDownloadLimit", s.opts.concurrentBlocksDownloadLimit).
		Msg("begin downloading blocks")

	downloadedBlocks := make(chan *blockchain.Block, s.opts.concurrentBlocksDownloadLimit)

	go func() {
		defer close(downloadedBlocks)
		defer s.opts.logger.Info().Msg("stopped downloading blocks")

		// The pub/sub for subscribe to the previous block download event
		// It is needed for make from disordered blocks chain to ordered after download of them
		// completed
		// So, the handler will handle whole blockchain in the right order block-by-block
		// withoout download speed reducing
		downloadedBlocksPubSub := pubsub.New(s.opts.blockHeadersBufferSize)
		defer downloadedBlocksPubSub.Shutdown()

		sentBlocks := sync.Map{}

		// Limiting the number of concurrent goroutines to download the blocks
		downloadBlocksSemaphore := semaphore.New(s.opts.concurrentBlocksDownloadLimit)

		for header := range headersCh {
			// Requesting a place fo the goroutine
			downloadBlocksSemaphore.Acquire()

			s.opts.logger.Info().
				Str("hash", header.GetHash().String()).
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
						blockEntity, err := s.restClient.GetBlock(ctx, header.GetHash())
						if err != nil {
							s.opts.logger.Error().
								Str("hash", header.GetHash().String()).
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
					Str("headerHash", header.GetHash().String()).
					Int64("size", block.GetSize()).
					Msg("downloaded the block")

				// Waiting for sending previous block if current block is no a genesis
				// and current block is not the first block we started from
				if len(header.GetPrevBlockHash().String()) != 0 && startedFrom.String() != header.GetHash().String() {
					if _, ok := sentBlocks.Load(header.GetPrevBlockHash().String()); !ok {
						<-downloadedBlocksPubSub.SubOnce(header.GetPrevBlockHash().String())
					}
				}

				select {
				case <-ctx.Done():
				default:
					// Sending the downloaded block only if process is not cancelled
					// Because this channel may be closed before the block is sent
					downloadedBlocks <- block
				}

				sentBlocks.Delete(header.GetPrevBlockHash().String())
				sentBlocks.Store(header.GetHash().String(), true)

				downloadedBlocksPubSub.TryPub(true, header.GetHash().String())
			}()
		}
	}()

	return downloadedBlocks
}

func (s *BitcoinBlocksIterator) downloadBlockHeaders(
	ctx context.Context,
	fromBlockHash blockchain.Hash,
	headersCh chan<- *blockchain.BlockHeader,
) {
	defer close(headersCh)

	s.opts.logger.Info().Str("fromBlockHash", fromBlockHash.String()).Int("blockHeadersBufferSize", s.opts.blockHeadersBufferSize).Msg(
		"begin downloading block headers",
	)

	currentBlockHash := fromBlockHash
	alreadySentLastHeader := false

	// First, we get the block headers in order to create a batch of blocks
	// Getting header is faster than getting full block information
	// So, after getting only headers, we can get all blocks parallely
	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.opts.logger.Info().Str("hash", currentBlockHash.String()).Msg("getting new header")

			header, err := s.restClient.GetBlockHeader(ctx, currentBlockHash)
			if err != nil {
				s.opts.logger.Error().Str("hash", currentBlockHash.String()).Err(err).Msg("failed to get block header")

				time.Sleep(s.opts.waitAfterErrorDuration)

				continue
			}

			newCurrentBlockHash := header.Hash

			if len(header.NextBlockHash) != 0 {
				newCurrentBlockHash = header.NextBlockHash
			}

			if !alreadySentLastHeader {
				headersCh <- header
				alreadySentLastHeader = true
			}

			// If there is new scanned block with different hash
			if slices.Compare(newCurrentBlockHash, currentBlockHash) != 0 {
				currentBlockHash = newCurrentBlockHash

				//  We need to send this header on the next iteration
				alreadySentLastHeader = false
			} else {
				s.opts.logger.Info().
					Dur("waitFor", s.opts.downloadHeadersInterval).
					Msg("there is no different next block header")

				time.Sleep(s.opts.downloadHeadersInterval)
			}
		}
	}
}

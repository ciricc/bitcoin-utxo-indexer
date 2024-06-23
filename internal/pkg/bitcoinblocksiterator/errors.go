package bitcoinblocksiterator

import "errors"

var (
	ErrInvalidBlockHeadersBufferSize        = errors.New("invalid block headers buffer size")
	ErrInvalidConcurrentBlocksDownloadLimit = errors.New("invalid concurrent blocks download limit")
)

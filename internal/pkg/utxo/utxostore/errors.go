package utxostore

import "errors"

var (
	ErrNotFound              = errors.New("data not found")
	ErrAlreadySpent          = errors.New("this output already spent")
	ErrIsPreviousBlockHeight = errors.New("the new block height >= current stored block height")
	ErrBlockHashNotFound     = errors.New("block hash not found")
)

package utxospending

import "errors"

var (
	ErrNoDiff                 = errors.New("there is no differences to change")
	ErrNextBlockTooFar        = errors.New("next block is too far from current (currentHeight + 1 != nextBlockHeight)")
	ErrSpendingOutputNotFound = errors.New("spending output is not found")
)

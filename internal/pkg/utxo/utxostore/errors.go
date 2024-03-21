package utxostore

import "errors"

var (
	ErrNotFound = errors.New("data not found")
	ErrAlreadySpent = errors.New("this output already spent")
)

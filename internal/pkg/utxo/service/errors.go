package utxoservice

import "errors"

var (
	ErrBlockAlreadyStored = errors.New("this block is already stored")
)

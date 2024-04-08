package utxoservice

import "errors"

var (
	ErrBlockAlreadyStored   = errors.New("this block is already stored")
	ErrInvalidBase58Address = errors.New("this is not a valid base-58 address")
)

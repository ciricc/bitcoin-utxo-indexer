package leveldbtx

import "errors"

var (
	ErrAlreadyCommitted = errors.New("transaction already committed")
)

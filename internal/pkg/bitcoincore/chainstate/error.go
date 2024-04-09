package chainstate

import "errors"

var (
	ErrNoKeysMore = errors.New("no more keys")
	ErrNotFound   = errors.New("not found")
)

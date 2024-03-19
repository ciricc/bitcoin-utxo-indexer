package restclient

import (
	"errors"
	"fmt"
)

var (
	ErrNodeHostNotSpecified = errors.New("no specified node host URL")
	ErrUnknownResponseType  = errors.New("unknown response type")
	ErrBlockNotFound        = errors.New("block not found")
	ErrBadStatusCode        = errors.New("bad status code")
)

func newBadStatusCodeError(statusCode int) error {
	return fmt.Errorf("bad status code (%d): %w", ErrBadStatusCode)
}

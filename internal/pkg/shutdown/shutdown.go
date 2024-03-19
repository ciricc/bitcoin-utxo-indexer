package shutdown

import (
	"io"

	"golang.org/x/sync/errgroup"
)

type Closeable interface {
	io.Closer
}

type CloseableNoError interface {
	Close()
}

type shutdownNoError struct {
	c CloseableNoError
}

func NewShutdownFromCloseableNoError(c CloseableNoError) *shutdownNoError {
	return &shutdownNoError{c: c}
}

func (s *shutdownNoError) Shutdown() error {
	s.c.Close()

	return nil
}

type shutdown struct {
	c Closeable
}

func NewShutdownFromCloseable(c Closeable) *shutdown {
	return &shutdown{c: c}
}

func (s *shutdown) Shutdown() error {
	return s.c.Close()
}

type Shutdownable interface {
	Shutdown() error
}

type Shutdowner struct {
	toShutdown []Shutdownable
}

func NewShutdowner(toShutdown ...Shutdownable) *Shutdowner {
	return &Shutdowner{toShutdown: toShutdown}
}

func (s *Shutdowner) Shutdown() error {
	groupErr := errgroup.Group{}

	for _, c := range s.toShutdown {
		groupErr.Go(c.Shutdown)
	}

	return groupErr.Wait()
}

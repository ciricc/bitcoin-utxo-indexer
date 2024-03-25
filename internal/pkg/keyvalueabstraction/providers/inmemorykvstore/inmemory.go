package inmemorykvstore

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/philippgille/gokv/encoding"
)

type Store struct {
	s map[string][]byte

	mx sync.RWMutex

	encoding encoding.Codec

	closure chan struct{}
}

// WithTx implements keyvaluestore.StoreWithTxManager.
func (i *Store) WithTx(tx txmanager.Transaction[*Store]) (keyvaluestore.StoreWithTxManager[*Store], error) {
	return i, nil
}

type StoreOptions struct {
	perstienceFilePath  string
	persistenceInterval time.Duration
}

type StoreOption func(s *StoreOptions) error

func WithPersistencePath(path string) StoreOption {
	return func(s *StoreOptions) error {
		s.perstienceFilePath = path

		return nil
	}
}

func WithPersistenceInterval(interval time.Duration) StoreOption {
	return func(s *StoreOptions) error {
		s.persistenceInterval = interval

		return nil
	}
}

func New(opts ...StoreOption) (*Store, error) {
	options := &StoreOptions{
		perstienceFilePath:  "",
		persistenceInterval: time.Second * 30,
	}

	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	s := &Store{
		encoding: encoding.JSON,
		s:        map[string][]byte{},
		mx:       sync.RWMutex{},
		closure:  make(chan struct{}, 1),
	}

	if len(options.perstienceFilePath) != 0 {
		if err := s.loadFromFileAndRunPersistence(
			options.perstienceFilePath,
			options.persistenceInterval,
		); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (i *Store) Close() error {
	select {
	case <-i.closure:
		return fmt.Errorf("already closed")
	default:
		i.closure <- struct{}{}
		close(i.closure)
	}

	return nil
}

func (i *Store) loadFromFileAndRunPersistence(path string, persistenceIntrerval time.Duration) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.ModePerm|os.ModeExclusive)
	if err != nil {
		return fmt.Errorf("open storage error: %w", err)
	}

	gobDecoder := gob.NewDecoder(f)
	i.mx.Lock()

	if err := gobDecoder.Decode(&i.s); err != nil {
		if !errors.Is(err, io.EOF) {
			i.mx.Unlock()
			return fmt.Errorf("decode with gob error: %w", err)
		}
	}

	i.mx.Unlock()

	go func() {
		t := time.NewTicker(persistenceIntrerval)

		defer t.Stop()
		defer f.Close()

		for {
			select {
			case <-t.C:
				// persist the
				i.saveStore(f)
			case <-i.closure:
				// persist the storage last time
				i.saveStore(f)
				return
			}
		}
	}()

	return err
}

func (i *Store) saveStore(f *os.File) error {
	i.mx.Lock()
	defer i.mx.Unlock()

	err := f.Truncate(0)
	if err != nil {
		return fmt.Errorf("truncate file error: %w", err)
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("seek error: %w", err)
	}

	encoder := gob.NewEncoder(f)
	err = encoder.Encode(&i.s)
	if err != nil {
		return fmt.Errorf("encode error: %w", err)
	}

	return nil
}

// Delete implements keyvaluestore.Store.
func (i *Store) Delete(key string) error {
	i.mx.Lock()
	defer i.mx.Unlock()

	delete(i.s, key)

	return nil
}

// Get implements keyvaluestore.Store.
func (i *Store) Get(key string, v any) (found bool, err error) {
	i.mx.RLock()
	defer i.mx.RUnlock()

	val, ok := i.s[key]
	if !ok {
		return false, nil
	}

	if err := i.encoding.Unmarshal(val, v); err != nil {
		return true, fmt.Errorf("unmarshal error: %w", err)
	}

	return true, nil
}

// ListKeys implements keyvaluestore.Store.
func (i *Store) ListKeys(si func(key string, getValue func(v interface{}) error) (stop bool, err error)) error {
	if si == nil {
		return nil
	}

	i.mx.RLock()

	// do not forget to unlock next or post-last key
	defer i.mx.RUnlock()

	for key, value := range i.s {
		i.mx.RUnlock()

		stop, err := si(key, func(v interface{}) error {
			if err := i.encoding.Unmarshal(value, v); err != nil {
				return fmt.Errorf("unmarshal error: %w", err)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("iterate error: %w", err)
		}

		if stop {
			break
		}

		// locks next (or post-last key)
		i.mx.RLock()
	}

	return nil
}

// Set implements keyvaluestore.Store.
func (i *Store) Set(key string, v any) error {
	i.mx.Lock()
	defer i.mx.Unlock()

	value, err := i.encoding.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	i.s[key] = value

	return nil
}

var _ keyvaluestore.StoreWithTxManager[*Store] = (*Store)(nil)

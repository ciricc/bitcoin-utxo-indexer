package keyvaluestore

import (
	"errors"
	"fmt"

	"github.com/philippgille/gokv/encoding"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type StoreOptions struct {
	Path string
}

type LevelDBStore struct {
	db *leveldb.DB

	encoding encoding.Codec
}

func NewLevelDBStore(options *StoreOptions) (*LevelDBStore, error) {
	db, err := leveldb.OpenFile(options.Path, &opt.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to open leveldb storage: %w", err)
	}

	return &LevelDBStore{
		db:       db,
		encoding: encoding.JSON,
	}, nil
}

func (l *LevelDBStore) Get(key string, v any) (found bool, err error) {
	val, err := l.db.Get([]byte(key), &opt.ReadOptions{})
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	if err := l.encoding.Unmarshal(val, v); err != nil {
		return true, fmt.Errorf("failed to decode value: %w", err)
	}

	return true, nil
}

func (l *LevelDBStore) Set(key string, v any) error {
	value, err := l.encoding.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to encode value: %w", err)
	}

	err = l.db.Put([]byte(key), value, &opt.WriteOptions{
		Sync: true,
	})
	if err != nil {
		return fmt.Errorf("failed to put the key: %w", err)
	}

	return nil
}

func (l *LevelDBStore) ListKeys(si func(key string, getValue func(v interface{}) error) (ok bool, err error)) error {
	iterator := l.db.NewIterator(nil, nil)
	defer iterator.Release()

	for iterator.Next() {
		key := iterator.Key()

		if si != nil {
			stop, err := si(string(key), func(v interface{}) error {
				value := iterator.Value()

				return l.encoding.Unmarshal(value, v)
			})
			if err != nil {
				return err
			}

			if stop {
				iterator.Release()
			}
		}
	}

	if err := iterator.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}

	return nil
}

package leveldbkvstore

import (
	"errors"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/philippgille/gokv/encoding"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelDB interface {
	// Delete deletes the value for the given key. It returns leveldb.ErrNotFound if the key does not exist.
	Delete(key []byte, wo *opt.WriteOptions) error

	// Get gets the value for the given key. It returns leveldb.ErrNotFound if the key does not exist.
	Get(key []byte, ro *opt.ReadOptions) (value []byte, err error)

	// NewIterator returns an iterator for the latest state of the database. The iterator is not safe for concurrent use.
	NewIterator(slice *util.Range, ro *opt.ReadOptions) iterator.Iterator

	// Put sets the value for the given key. It overwrites any previous value for that key.
	Put(key []byte, value []byte, wo *opt.WriteOptions) error
}

type StoreOptions struct {
	// Path to the LevelDB database (folder)
	Path string
}

type LevelDBStore struct {
	db       LevelDB
	encoding encoding.Codec
}

func NewLevelDBStore(db LevelDB) (*LevelDBStore, error) {
	return &LevelDBStore{
		db:       db,
		encoding: encoding.JSON,
	}, nil
}

func (l *LevelDBStore) Get(key string, v any) (found bool, err error) {
	val, err := l.db.Get([]byte(key), nil)
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

	err = l.db.Put([]byte(key), value, nil)
	if err != nil {
		return fmt.Errorf("failed to put the key: %w", err)
	}

	return nil
}

func (l *LevelDBStore) Delete(key string) error {
	err := l.db.Delete([]byte(key), nil)
	if err != nil {
		return fmt.Errorf("failed to delete the key: %w", err)
	}

	return nil
}

func (l *LevelDBStore) WithTx(tx txmanager.Transaction[*leveldb.Transaction]) (keyvaluestore.StoreWithTxManager[*leveldb.Transaction], error) {
	return &LevelDBStore{
		db:       tx.Transaction(),
		encoding: l.encoding,
	}, nil
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

var _ keyvaluestore.StoreWithTxManager[*leveldb.Transaction] = (*LevelDBStore)(nil)

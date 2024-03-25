package redistx

import (
	"context"
	"testing"
	"time"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/transactionmanager/txmanager"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

const OK = "OK"

func TestTransaction(t *testing.T) {
	t.Parallel()

	type args struct {
		ctx context.Context
	}

	ctx := context.Background()
	// testErr := errors.New("error test")
	testKey := "key1"
	testValue := "value"
	testExp := time.Duration(0)

	tests := map[string]struct {
		prepare  func(t *testing.T, m redismock.ClientMock)
		args     args
		ret      error
		wantErr  assert.ErrorAssertionFunc
		wantCmds int
	}{
		"commit": {
			prepare: func(_ *testing.T, m redismock.ClientMock) {
				// m.ExpectWatch(testKey)
				m.ExpectTxPipeline()

				m.ExpectSet(testKey, testValue, testExp).SetVal(OK)

				m.ExpectTxPipelineExec()
			},
			args: args{
				ctx: ctx,
			},
			ret:      nil,
			wantErr:  assert.NoError,
			wantCmds: 1,
		},
		// "begin_error": {
		// 	prepare: func(_ *testing.T, _ redismock.ClientMock) {},
		// 	args: args{
		// 		ctx: ctx,
		// 	},
		// 	wantErr: func(t assert.TestingT, err error, _ ...interface{}) bool {
		// 		return assert.ErrorContains(t, err, "all expectations were already fulfilled, call to cmd '[watch key1]' was not expected") &&
		// 			assert.ErrorIs(t, err, trm.ErrBegin)
		// 	},
		// },
		// "commit_error": {
		// 	prepare: func(_ *testing.T, m redismock.ClientMock) {
		// 		// m.ExpectWatch(testKey)
		// 		m.ExpectTxPipeline()

		// 		m.ExpectSet(testKey, testValue, testExp).SetVal(OK)

		// 		m.ExpectTxPipelineExec().RedisNil()
		// 	},
		// 	args: args{
		// 		ctx: ctx,
		// 	},
		// 	ret: nil,
		// 	wantErr: func(t assert.TestingT, err error, _ ...interface{}) bool {
		// 		return assert.ErrorContains(t, err, "redis: nil") &&
		// 			assert.ErrorIs(t, err, trm.ErrCommit)
		// 	},
		// 	wantCmds: 1,
		// },
		// "rollback": {
		// 	prepare: func(_ *testing.T, m redismock.ClientMock) {
		// 		// m.ExpectWatch(testKey)
		// 	},
		// 	args: args{
		// 		ctx: ctx,
		// 	},
		// 	ret: testErr,
		// 	wantErr: func(t assert.TestingT, err error, _ ...interface{}) bool {
		// 		return assert.ErrorIs(t, err, testErr)
		// 	},
		// },
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			db, rmock := redismock.NewClientMock()

			tt.prepare(t, rmock)

			m := txmanager.New(NewRedisTransactionFactory(db))

			err := m.Do(func(ctx context.Context, tx txmanager.Transaction[*redis.Tx]) error {

				cmd := tx.Transaction().Set(ctx, testKey, testValue, testExp)
				if cmd.Err() != nil {
					return cmd.Err()
				}

				return nil
			})

			if !tt.wantErr(t, err) {
				return
			}

			// assert.Len(t, s.Return(), tt.wantCmds)
			assert.NoError(t, rmock.ExpectationsWereMet())
		})
	}
}

// func TestTransaction_awaitDone_byContext(t *testing.T) {
// 	t.Parallel()

// 	wg := sync.WaitGroup{}
// 	wg.Add(1)

// 	db, rmock := redismock.NewClientMock()

// 	f := NewDefaultFactory(db)
// 	ctx, cancel := context.WithCancel(context.Background())

// 	go func() {
// 		defer wg.Done()

// 		_, tr, err := f(ctx, settings.Must())
// 		require.NoError(t, err)

// 		cancel()

// 		<-ctx.Done()
// 		require.True(t, tr.IsActive())
// 		<-tr.Closed()
// 		require.False(t, tr.IsActive())

// 		require.Equal(t, context.Canceled, ctx.Err())
// 		err = tr.Commit(ctx)
// 		require.ErrorIs(t, err, redis.ErrClosed)
// 	}()

// 	wg.Wait()
// 	assert.NoError(t, rmock.ExpectationsWereMet())
// }

// // TestTransaction_awaitDone_byRollback checks goroutine leak when we close transaction manually.
// func TestTransaction_awaitDone_byRollback(t *testing.T) {
// 	t.Parallel()

// 	db, rmock := redismock.NewClientMock()

// 	f := NewDefaultFactory(db)
// 	ctx, _ := context.WithCancel(context.Background()) //nolint:govet

// 	wg := sync.WaitGroup{}
// 	wg.Add(1)

// 	go func() {
// 		defer wg.Done()

// 		_, tr, err := f(ctx, settings.Must())
// 		require.NoError(t, err)

// 		require.NoError(t, tr.Rollback(ctx))
// 		require.False(t, tr.IsActive())
// 		require.NoError(t, tr.Rollback(ctx))
// 	}()

// 	wg.Wait()
// 	assert.NoError(t, rmock.ExpectationsWereMet())
// }

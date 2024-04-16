package redistx

import (
	"context"
	"errors"
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
	testErr := errors.New("error test")
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
		"begin_error": {
			prepare: func(_ *testing.T, _ redismock.ClientMock) {},
			args: args{
				ctx: ctx,
			},
			wantErr: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.ErrorContains(t, err, "all expectations were already fulfilled, call to cmd '[multi]' was not expected")
			},
			wantCmds: 0,
		},
		"commit_error": {
			prepare: func(_ *testing.T, m redismock.ClientMock) {
				// m.ExpectWatch(testKey)
				m.ExpectTxPipeline()

				m.ExpectSet(testKey, testValue, testExp).SetVal(OK)

				m.ExpectTxPipelineExec().RedisNil()
			},
			args: args{
				ctx: ctx,
			},
			ret: nil,
			wantErr: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.ErrorContains(t, err, "redis: nil")
			},
			wantCmds: 0,
		},
		"rollback": {
			args: args{
				ctx: ctx,
			},
			prepare: func(t *testing.T, m redismock.ClientMock) {},
			ret:     testErr,
			wantErr: func(t assert.TestingT, err error, _ ...interface{}) bool {
				return assert.ErrorIs(t, err, testErr)
			},
			wantCmds: 0,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			db, rmock := redismock.NewClientMock()

			tt.prepare(t, rmock)

			m := txmanager.New(NewRedisTransactionFactory(db))

			err := m.Do(nil, func(ctx context.Context, tx txmanager.Transaction[redis.Pipeliner]) error {

				cmd := tx.Transaction().Set(ctx, testKey, testValue, testExp)
				if cmd.Err() != nil {
					return cmd.Err()
				}

				return tt.ret
			})

			if !tt.wantErr(t, err) {
				return
			}

			assert.NoError(t, rmock.ExpectationsWereMet())
		})
	}
}

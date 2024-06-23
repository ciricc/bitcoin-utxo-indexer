package utxostore

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
)

type redisAddressUTXOIdx[T any] struct {
	s     sets.SetsWithTxManager[T]
	dbVer string
}

func newAddressUTXOIndex[T any](databaseVersion string, sets sets.SetsWithTxManager[T]) *redisAddressUTXOIdx[T] {
	return &redisAddressUTXOIdx[T]{
		s:     sets,
		dbVer: databaseVersion,
	}
}

func (u *redisAddressUTXOIdx[T]) deleteAdressUTXOTransactionIds(ctx context.Context, address string, txIDs []string) error {
	addressUTXOTxIDsKey := newAddressUTXOTxIDsSetKey(u.dbVer, address)

	err := u.s.RemoveFromSet(ctx, addressUTXOTxIDsKey.String(), txIDs...)
	if err != nil {
		return fmt.Errorf("failed to delete address UTXO tx ids: %w", err)
	}

	return nil
}

func (i *redisAddressUTXOIdx[T]) getAddressUTXOTransactionIds(ctx context.Context, address string) ([]string, error) {
	addressUTXOTxIDsKey := newAddressUTXOTxIDsSetKey(i.dbVer, address)
	txIds, err := i.s.GetSet(ctx, addressUTXOTxIDsKey.String())
	if err != nil {
		return nil, fmt.Errorf("get address UTXO tx ids set error: %w", err)
	}

	return txIds, nil
}

func (i *redisAddressUTXOIdx[T]) addAddressUTXOTransactionIds(
	ctx context.Context,
	address string,
	txIDs []string,
) error {
	err := i.s.AddToSet(ctx, newAddressUTXOTxIDsSetKey(i.dbVer, address).String(), txIDs...)
	if err != nil {
		return fmt.Errorf("failed to add address UTXO tx ids: %w", err)
	}

	return nil
}

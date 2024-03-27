package utxostore

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
)

type redisAddressUTXOIdx struct {
	s sets.Sets
}

func newAddressUTXOIndex(sets sets.Sets) *redisAddressUTXOIdx {
	return &redisAddressUTXOIdx{
		s: sets,
	}
}

func (u *redisAddressUTXOIdx) deleteAdressUTXOTransactionIds(address string, txIDs []string) error {
	addressUTXOTxIDsKey := newAddressUTXOTxIDsSetKey(address)

	err := u.s.RemoveFromSet(context.Background(), addressUTXOTxIDsKey.String(), txIDs...)
	if err != nil {
		return fmt.Errorf("failed to delete address UTXO tx ids: %w", err)
	}

	return nil
}

func (i *redisAddressUTXOIdx) getAddressUTXOTransactionIds(address string) ([]string, error) {
	addressUTXOTxIDsKey := newAddressUTXOTxIDsSetKey(address)
	txIds, err := i.s.GetSet(context.Background(), addressUTXOTxIDsKey.String())
	if err != nil {
		return nil, fmt.Errorf("get address UTXO tx ids set error: %w", err)
	}

	return txIds, nil
}

func (i *redisAddressUTXOIdx) addAddressUTXOTransactionIds(
	address string,
	txIDs []string,
) error {
	err := i.s.AddToSet(context.Background(), newAddressUTXOTxIDsSetKey(address).String(), txIDs...)
	if err != nil {
		return fmt.Errorf("failed to add address UTXO tx ids: %w", err)
	}

	return nil
}

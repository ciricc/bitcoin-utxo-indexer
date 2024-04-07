package utxostore

import (
	"context"
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/setsabstraction/sets"
)

type redisAddressUTXOIdx struct {
	s     sets.Sets
	dbVer string
}

func newAddressUTXOIndex(databaseVersion string, sets sets.Sets) *redisAddressUTXOIdx {
	return &redisAddressUTXOIdx{
		s:     sets,
		dbVer: databaseVersion,
	}
}

func (u *redisAddressUTXOIdx) deleteAdressUTXOTransactionIds(address string, txIDs []string) error {
	addressUTXOTxIDsKey := newAddressUTXOTxIDsSetKey(u.dbVer, address)

	err := u.s.RemoveFromSet(context.Background(), addressUTXOTxIDsKey.String(), txIDs...)
	if err != nil {
		return fmt.Errorf("failed to delete address UTXO tx ids: %w", err)
	}

	return nil
}

func (i *redisAddressUTXOIdx) getAddressUTXOTransactionIds(address string) ([]string, error) {
	addressUTXOTxIDsKey := newAddressUTXOTxIDsSetKey(i.dbVer, address)
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
	err := i.s.AddToSet(context.Background(), newAddressUTXOTxIDsSetKey(i.dbVer, address).String(), txIDs...)
	if err != nil {
		return fmt.Errorf("failed to add address UTXO tx ids: %w", err)
	}

	return nil
}

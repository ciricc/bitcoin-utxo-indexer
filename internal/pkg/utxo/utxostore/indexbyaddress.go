package utxostore

import (
	"fmt"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/keyvalueabstraction/keyvaluestore"
)

type addressUTXOIdx struct {
	s keyvaluestore.Store
}

func newAddressUTXOIndex(storer keyvaluestore.Store) *addressUTXOIdx {
	return &addressUTXOIdx{
		s: storer,
	}
}

func (u *addressUTXOIdx) deleteAdressUTXOTransactionIds(address string) error {
	txIds, err := u.getAddressUTXOTransactionIds(address)
	if err != nil {
		return fmt.Errorf("failed to get address UTXO tx ids: %w", err)
	}

	for _, txID := range txIds {
		addressUTXOTxIDsKey := newAddressUTXOTxIDsKey(address, txID)
		if err := u.s.Delete(addressUTXOTxIDsKey.String()); err != nil {
			return fmt.Errorf("failed to remove idx: %w", err)
		}
	}

	return nil
}

func (i *addressUTXOIdx) getAddressUTXOTransactionIds(address string) ([]string, error) {

	txIds := []string{}
	addressUTXOTxIDsKey := newAddressUTXOTxIDsKey(address, "*")

	err := i.s.ListKeys(addressUTXOTxIDsKey.String(), func(key string, getValue func(v interface{}) error) (ok bool, err error) {
		txIds = append(txIds, key)

		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	return txIds, nil
}

func (i *addressUTXOIdx) addAddressUTXOTransactionIds(
	address string,
	txIDs []string,
) error {
	for _, txID := range txIDs {
		addressUTXOTxIDsKey := newAddressUTXOTxIDsKey(address, txID)

		err := i.s.Set(addressUTXOTxIDsKey.String(), 0)
		if err != nil {
			return fmt.Errorf("failed to store address UTXO tx ids index: %w", err)
		}
	}

	return nil
}

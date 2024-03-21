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
	addressUTXOTxIDsKey := newAddressUTXOTxIDsKey(address)
	if err := u.s.Delete(addressUTXOTxIDsKey.String()); err != nil {
		return fmt.Errorf("failed to remove idx: %w", err)
	}

	return nil
}

func (i *addressUTXOIdx) getAddressUTXOTransactionIds(address string) ([]string, error) {
	indexedTxs := map[string]int8{}

	addressUTXOTxIDsKey := newAddressUTXOTxIDsKey(address)

	found, err := i.s.Get(addressUTXOTxIDsKey.String(), &indexedTxs)
	if err != nil {
		return nil, fmt.Errorf("failed to get address UTXO tx ids")
	}

	if !found {
		return nil, nil
	}

	txIds := make([]string, 0, len(indexedTxs))

	for txId := range indexedTxs {
		txIds = append(txIds, txId)
	}

	return txIds, nil
}

func (i *addressUTXOIdx) addAddressUTXOTransactionIds(
	address string,
	txIDs []string,
) error {

	indexedTxs := map[string]int8{}

	addressUTXOTxIDsKey := newAddressUTXOTxIDsKey(address)

	_, err := i.s.Get(addressUTXOTxIDsKey.String(), &indexedTxs)
	if err != nil {
		return fmt.Errorf("failed to get address UTXO tx ids")
	}

	for _, txID := range txIDs {
		indexedTxs[txID] = 0
	}

	err = i.s.Set(addressUTXOTxIDsKey.String(), indexedTxs)
	if err != nil {
		return fmt.Errorf("failed to store address UTXO tx ids index: %w", err)
	}

	return nil
}

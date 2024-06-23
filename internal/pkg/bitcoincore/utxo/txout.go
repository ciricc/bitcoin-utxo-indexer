package utxo

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/btcsuite/btcd/wire"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/binaryutils"
)

type coinMarshaled struct {
	IsCoinbase bool        `json:"coinbase"`
	Output     *wire.TxOut `json:"output"`
}

type txOutMarshaled struct {
	Coin  *coinMarshaled `json:"coin"`
	TxID  string         `json:"txId"`
	Index uint64         `json:"index"`
}

type TxOut struct {
	txID  []byte
	index uint64
	coin  *Coin
}

func NewTxOut() *TxOut {
	return &TxOut{
		coin: NewCoin(),
	}
}

func (t *TxOut) GetCoin() *Coin {
	return t.coin
}

func (t *TxOut) GetTxID() string {
	return hex.EncodeToString(t.txID[:])
}

func (t *TxOut) Index() uint64 {
	return t.index
}

func (t *TxOut) MarshalJSON() ([]byte, error) {
	coin := t.GetCoin()
	tJson := txOutMarshaled{
		Coin: &coinMarshaled{
			Output:     coin.GetOut(),
			IsCoinbase: coin.IsCoinbase(),
		},
		TxID:  t.GetTxID(),
		Index: t.index,
	}

	return json.Marshal(&tJson)
}

func (t *TxOut) Deserialize(r BytesBuffer) error {
	txID := make([]byte, 32)
	_, err := r.Read(txID)
	if err != nil {
		return fmt.Errorf("read txID error: %w", err)
	}

	txID = binaryutils.ReverseBytesWithCopy(txID)
	t.txID = txID

	index, _, err := binaryutils.DeserializeVLQ(r)
	if err != nil {
		return fmt.Errorf("read index error: %w", err)
	}

	t.index = index

	return t.coin.Deserialize(r)
}

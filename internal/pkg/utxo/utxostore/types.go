package utxostore

import (
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bigjson"
)

type TransactionOutput struct {
	ScriptBytes string           `json:"0"`
	Amount      bigjson.BigFloat `json:"1"`
	Addresses   []string         `json:"2"`
}

type UTXOEntry struct {
	TxID   string
	Vout   uint32
	Output *TransactionOutput
}

type addressOutputs map[string][]*TransactionOutput

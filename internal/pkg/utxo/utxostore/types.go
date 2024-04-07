package utxostore

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/utxocompression"
)

type TransactionOutput struct {
	ScriptBytes      []byte `json:"0" msgpack:"0"`
	CompressedAmount uint64 `json:"1" msgpack:"1"`
}

func (t *TransactionOutput) SetAmount(amount uint64) {
	t.CompressedAmount = utxocompression.CompressTxOutAmount(amount)
}

func (t *TransactionOutput) GetAmount() uint64 {
	return utxocompression.DecompressTxOutAmount(t.CompressedAmount)
}

func (t *TransactionOutput) GetAddresses() ([]string, error) {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(t.ScriptBytes, &chaincfg.MainNetParams)
	if err != nil {
		return nil, fmt.Errorf("failed to extract addresses from the script: %w", err)
	}

	if len(addrs) == 0 {
		return nil, nil
	}

	addrsStrings := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addrsStrings = append(addrsStrings, hex.EncodeToString(addr.ScriptAddress()))
	}

	return addrsStrings, nil
}

type UTXOEntry struct {
	TxID   string
	Vout   uint32
	Output *TransactionOutput
}

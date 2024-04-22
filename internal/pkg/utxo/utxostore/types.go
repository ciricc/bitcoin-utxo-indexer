package utxostore

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/bitcoincorecompression"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/utxocompression"
	"github.com/shopspring/decimal"
)

type TransactionOutput struct {
	CompressedScript []byte `json:"0" msgpack:"0"`
	CompressedAmount uint64 `json:"1" msgpack:"1"`
}

func (t *TransactionOutput) SetAmount(amount uint64) {
	t.CompressedAmount = utxocompression.CompressTxOutAmount(amount)
}

func (t *TransactionOutput) GetAmount() uint64 {
	return utxocompression.DecompressTxOutAmount(t.CompressedAmount)
}

func (t *TransactionOutput) GetAmountFloat64(decimals int) (*big.Float, error) {
	amountInt := t.GetAmount()
	amountStr := strconv.FormatUint(amountInt, 10)

	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount to decimal: %w", err)
	}

	return amount.Div(decimal.New(1, int32(decimals))).BigFloat(), nil
}

func (t *TransactionOutput) GetScriptBytes() []byte {
	return bitcoincorecompression.DecompressScript(t.CompressedScript)
}

func (t *TransactionOutput) SetScriptBytes(pkScript []byte) {
	t.CompressedScript = make([]byte, len(pkScript)+16)
	size := bitcoincorecompression.PutCompressedScript(t.CompressedScript, pkScript)
	_ = size
	t.CompressedScript = t.CompressedScript[:size]
}

func (t *TransactionOutput) GetAddresses() ([]string, error) {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(t.GetScriptBytes(), &chaincfg.MainNetParams)
	if err != nil {
		return nil, fmt.Errorf("failed to extract addresses from the script: %w", err)
	}

	if len(addrs) == 0 {
		return nil, nil
	}

	addrsStrings := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		addressHash := addr.ScriptAddress()
		if pubKeyAddr, ok := addr.(*btcutil.AddressPubKey); ok {
			addressHash = pubKeyAddr.AddressPubKeyHash().ScriptAddress()
		}

		addrsStrings = append(addrsStrings, hex.EncodeToString(addressHash))
	}

	return addrsStrings, nil
}

type UTXOEntry struct {
	TxID   string
	Vout   uint32
	Output *TransactionOutput
}

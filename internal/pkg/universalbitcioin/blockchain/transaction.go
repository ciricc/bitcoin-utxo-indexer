package blockchain

import (
	"encoding/hex"
	"strconv"

	"github.com/btcsuite/btcd/txscript"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bigjson"
	"github.com/shopspring/decimal"
)

type AmountValue struct {
	bigjson.BigFloat
}

func (a *AmountValue) Uint64(decimals int) uint64 {
	mulBy := decimal.New(1, int32(decimals))

	amountF, _ := decimal.NewFromString(a.String())
	amountInt, _ := strconv.ParseUint(amountF.Mul(mulBy).String(), 10, 64)

	return amountInt
}

type CoinbaseInput struct {
	Coinbase string `json:"coinbase"`
}

func (c *CoinbaseInput) GetCoinbase() string {
	return c.Coinbase
}

type TransactionInput struct {
	*CoinbaseInput
	*SpendingOutput
	Sequence int64 `json:"sequence"`
}

func (t *TransactionInput) GetCoinbase() string {
	return t.Coinbase
}

type Transaction struct {
	ID       string               `json:"txid"`
	Hash     Hash                 `json:"hash"`
	LockTime int64                `json:"locktime"`
	Size     int                  `json:"size"`
	Version  int                  `json:"version"`
	VOut     []*TransactionOutput `json:"vout"`
	VIn      []*TransactionInput  `json:"vin"`
}

func (t *Transaction) GetID() string {
	return t.ID
}

func (t *Transaction) GetHash() Hash {
	return t.Hash
}

func (t *Transaction) GetLockTime() int64 {
	return t.LockTime
}

func (t *Transaction) GetSize() int {
	return t.Size
}

func (t *Transaction) GetVersion() int {
	return t.Version
}

func (t *Transaction) GetInputs() []*TransactionInput {
	return t.VIn
}

func (t *Transaction) GetOutputs() []*TransactionOutput {
	return t.VOut
}

type SpendingOutput struct {
	PrevOut   *PrevOut   `json:"prevout"`
	ScriptSig *ScriptSig `json:"scriptSig"`
	TxID      string     `json:"txid"`
	VOut      int        `json:"vout"`
}

func (s *SpendingOutput) GetPrevOut() *PrevOut {
	return s.PrevOut
}

func (s *SpendingOutput) GetScriptSig() *ScriptSig {
	return s.ScriptSig
}

func (s *SpendingOutput) GetTxID() string {
	return s.TxID
}

func (s *SpendingOutput) GetVOut() int {
	return s.VOut
}

type TransactionOutput struct {
	N            int           `json:"n"`
	Value        *AmountValue  `json:"value"`
	ScriptPubKey *ScriptPubKey `json:"scriptPubKey"`
}

func (t *TransactionOutput) GetN() int {
	return t.N
}

func (t *TransactionOutput) GetValue() *AmountValue {
	return t.Value
}

func (t *TransactionOutput) GetScriptPubKey() *ScriptPubKey {
	return t.ScriptPubKey
}

func (t *TransactionOutput) IsSpendable() bool {
	if t.ScriptPubKey == nil {
		return false
	}

	if t.ScriptPubKey.Type == "nulldata" {
		return false
	}

	if len(t.ScriptPubKey.HEX) == 0 {
		return false
	}

	scriptBytes, err := hex.DecodeString(t.ScriptPubKey.HEX)
	if err != nil {
		return false
	}

	return !txscript.IsUnspendable(scriptBytes)
}

type PrevOut struct {
	Generated bool         `json:"generated"`
	Height    int64        `json:"height"`
	Value     *AmountValue `json:"value"`
}

func (p *PrevOut) GetGenerated() bool {
	return p.Generated
}

func (p *PrevOut) GetHeight() int64 {
	return p.Height
}

func (p *PrevOut) GetValue() *AmountValue {
	return p.Value
}

type ScriptPubKey struct {
	Address   string   `json:"address"`
	Addresses []string `json:"addresses"`
	ASM       string   `json:"asm"`
	HEX       string   `json:"hex"`
	Type      string   `json:"type"`
}

type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

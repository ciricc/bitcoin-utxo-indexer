package utxospending

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/bitcoincorecompression"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/stretchr/testify/require"
)

const zeroP2PKHHex = "0000000000000000000000000000000000000000"
const thirdAddressHashHex = "47f0c44dc09324ad49a21317cbc94c6652562c6a"

var zeroOutput *utxostore.TransactionOutput

type btcConfigMock struct{}

func (b *btcConfigMock) GetDecimals() int {
	return 8
}

func buildP2PKHScript(hash string) string {
	return fmt.Sprintf("76a914%s88ac", hash)
}

func newOutputWithP2PKH(p2pkhash string) (*utxostore.TransactionOutput, error) {
	return newOutputWithScriptPkHex(buildP2PKHScript(p2pkhash))
}

func newOutputWithScriptPkHex(scriptPkHEX string) (*utxostore.TransactionOutput, error) {
	pkScript, err := hex.DecodeString(scriptPkHEX)
	if err != nil {
		return nil, fmt.Errorf("failed to decode script hex: %w", err)
	}

	compressedScriptBytes := make([]byte, 512)
	scriptSize := bitcoincorecompression.PutCompressedScript(
		compressedScriptBytes,
		pkScript,
	)
	compressedScriptBytes = compressedScriptBytes[:scriptSize]

	return &utxostore.TransactionOutput{
		CompressedScript: compressedScriptBytes,
		CompressedAmount: uint64(bitcoincorecompression.CompressTxOutAmount(1)),
	}, nil
}

func TestMain(t *testing.M) {
	var err error

	zeroOutput, err = newOutputWithP2PKH(zeroP2PKHHex)
	if err != nil {
		panic(err)
	}

	addresses, err := zeroOutput.GetAddresses()
	if err != nil {
		panic(err)
	}

	if len(addresses) == 0 {
		panic("zero output has no any address")
	}

	t.Run()
}

func TestExtractAddressReferencesFromOutputs(t *testing.T) {
	addressReferences, err := extractAddressReferencesFromOutputs(&btcConfigMock{}, &blockchain.Block{
		Transactions: []*blockchain.Transaction{
			{
				ID: "1",
				VOut: []*blockchain.TransactionOutput{
					{
						N: 0,
						ScriptPubKey: &blockchain.ScriptPubKey{
							HEX: buildP2PKHScript(zeroP2PKHHex),
						},
					},
					{
						N: 1,
						ScriptPubKey: &blockchain.ScriptPubKey{
							HEX: buildP2PKHScript(zeroP2PKHHex),
						},
					},
				},
			},
			{
				ID: "2",
				VOut: []*blockchain.TransactionOutput{
					{
						N: 0,
						ScriptPubKey: &blockchain.ScriptPubKey{
							HEX: buildP2PKHScript(zeroP2PKHHex),
						},
					},
					{
						N: 1,
						ScriptPubKey: &blockchain.ScriptPubKey{
							HEX: buildP2PKHScript(zeroP2PKHHex),
						},
					},
					{
						N: 1,
						ScriptPubKey: &blockchain.ScriptPubKey{
							HEX: buildP2PKHScript(thirdAddressHashHex),
						},
					},
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, map[string][]string{
		zeroP2PKHHex:        {"1", "2"},
		thirdAddressHashHex: {"2"},
	}, addressReferences)
}

func TestSpendOutputs(t *testing.T) {
	oneSatAmount, err := blockchain.NewAmountValueFromString("0.00000001")
	require.NoError(t, err)

	newOutputs, err := spendAllOutputs(&btcConfigMock{}, map[string]*utxostore.TransactionOutputs{
		"1": {
			BlockHeight: 1,
			Outputs: []*utxostore.TransactionOutput{
				zeroOutput,
			},
		},
	}, &blockchain.Block{
		BlockHeader: blockchain.BlockHeader{
			Height: 2,
		},
		Transactions: []*blockchain.Transaction{
			{
				ID: "2",
				VIn: []*blockchain.TransactionInput{
					{
						SpendingOutput: &blockchain.SpendingOutput{
							TxID: "1",
							VOut: 0,
						},
					},
				},
				VOut: []*blockchain.TransactionOutput{
					{
						N:     0,
						Value: oneSatAmount,
						ScriptPubKey: &blockchain.ScriptPubKey{
							HEX: buildP2PKHScript(zeroP2PKHHex),
						},
					},
				},
			},
		},
	})

	require.NoError(t, err)

	require.Equal(t, map[string]*utxostore.TransactionOutputs{
		"1": {
			BlockHeight: 1,
			Outputs:     []*utxostore.TransactionOutput{nil},
		},
		"2": {
			BlockHeight: 2,
			Outputs:     []*utxostore.TransactionOutput{zeroOutput},
		},
	}, newOutputs)
}

func TestAddressCountOutputs(t *testing.T) {
	anotherOutput, err := newOutputWithP2PKH(thirdAddressHashHex)
	require.NoError(t, err)

	addressOutputsCount, err := countAddressOutputs(map[string]*utxostore.TransactionOutputs{
		"1": {
			BlockHeight: 1,
			Outputs: []*utxostore.TransactionOutput{
				anotherOutput,
				zeroOutput,
			},
		},
		"2": {
			BlockHeight: 1,
			Outputs: []*utxostore.TransactionOutput{
				nil,
				zeroOutput,
				zeroOutput,
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, map[string]map[string]int{
		thirdAddressHashHex: {
			"1": 1,
		},
		zeroP2PKHHex: {
			"2": 2,
			"1": 1,
		},
	}, addressOutputsCount)
}

func TestDereferenceAddress(t *testing.T) {
	require.NotNil(t, zeroOutput)

	thirdOutput, err := newOutputWithP2PKH(thirdAddressHashHex)
	require.NoError(t, err)

	dereferencedAddresses, err := derefAddresses(map[string]*utxostore.TransactionOutputs{
		"1": {
			BlockHeight: 1,
			Outputs: []*utxostore.TransactionOutput{
				zeroOutput,
			},
		},
		"2": {
			BlockHeight: 1,
			Outputs: []*utxostore.TransactionOutput{
				zeroOutput,
				thirdOutput,
			},
		},
	}, &blockchain.Block{
		Transactions: []*blockchain.Transaction{
			{
				ID: "3",
				VIn: []*blockchain.TransactionInput{
					{
						SpendingOutput: &blockchain.SpendingOutput{
							TxID:    "1",
							VOut:    0,
							PrevOut: &blockchain.PrevOut{},
						},
					},
					{
						SpendingOutput: &blockchain.SpendingOutput{
							TxID:    "2",
							VOut:    1,
							PrevOut: &blockchain.PrevOut{},
						},
					},
				},
			},
		},
	})

	require.NoError(t, err)

	require.Equal(t, map[string][]string{
		zeroP2PKHHex:        {"1"},
		thirdAddressHashHex: {"2"},
	}, dereferencedAddresses)
}

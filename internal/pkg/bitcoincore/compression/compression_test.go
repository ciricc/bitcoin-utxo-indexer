package blockchain

import (
	"encoding/hex"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecompressTxOut(t *testing.T) {
	txOutBytes, err := hex.DecodeString("35f7fcad13ef02851d973dc5ba9136a4c211693b5d0954cd725ab602")

	code, offset := deserializeVLQ(txOutBytes)
	log.Println("code", code)
	txOutBytes = txOutBytes[offset:]

	require.NoError(t, err)

	_, _, _, err = decodeCompressedTxOut(txOutBytes)
	require.NoError(t, err)
}

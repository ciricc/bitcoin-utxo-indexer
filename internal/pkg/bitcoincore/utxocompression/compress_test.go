package utxocompression

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecompressAmount(t *testing.T) {
	decompressedAmount := DecompressTxOutAmount(80643)
	require.Equal(t, uint64(0), int64(decompressedAmount))
}

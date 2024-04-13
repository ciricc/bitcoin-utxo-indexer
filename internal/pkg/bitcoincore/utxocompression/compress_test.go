package utxocompression

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecompressAmount(t *testing.T) {
	decompressedAmount := DecompressTxOutAmount(80772)
	require.Equal(t, uint64(89750), decompressedAmount)
}

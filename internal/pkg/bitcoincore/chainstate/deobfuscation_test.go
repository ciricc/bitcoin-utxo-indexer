package chainstate

import (
	"encoding/hex"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestDeobfuscateValue(t *testing.T) {
	testCases := []struct {
		obfuscationKey []byte
		value          []byte
		expectedValue  []byte
	}{
		{
			obfuscationKey: lo.Must(hex.DecodeString("27c78118b7316105")),
			value:          lo.Must(hex.DecodeString("26c326d7353661dc7005d274976f458691f24f0f05d141335f4ad5927e41")),
			expectedValue:  lo.Must(hex.DecodeString("0104a7cf820700d957c2536c205e2483b635ce17b2e02036788d548ac970")),
		},
		{
			obfuscationKey: lo.Must(hex.DecodeString("08abc8545e82efea6f")),
			value:          lo.Must(hex.DecodeString("3b690c4f8214a76e87862b138f7f14e9810f3d3f6ecd9eaf6e")),
			expectedValue:  lo.Must(hex.DecodeString("")),
		},
	}

	for _, tc := range testCases {
		val := deobfuscateValue(tc.obfuscationKey, tc.value)
		assert.Equal(t, hex.EncodeToString(val), hex.EncodeToString(tc.expectedValue))
	}

}

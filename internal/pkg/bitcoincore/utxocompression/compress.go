package utxocompression

// Copyright (c) 2015-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/txscript"
)

// The following constants specify the special constants used to identify a
// special script type in the domain-specific compressed script encoding.
//
// NOTE: This section specifically does not use iota since these values are
// serialized and must be stable for long-term storage.
const (
	// CstPayToPubKeyHash identifies a compressed pay-to-pubkey-hash script.
	CstPayToPubKeyHash = 0

	// CstPayToScriptHash identifies a compressed pay-to-script-hash script.
	CstPayToScriptHash = 1

	// CstPayToPubKeyComp2 identifies a compressed pay-to-pubkey script to
	// a compressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	CstPayToPubKeyComp2 = 2

	// CstPayToPubKeyComp3 identifies a compressed pay-to-pubkey script to
	// a compressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	CstPayToPubKeyComp3 = 3

	// CstPayToPubKeyUncomp4 identifies a compressed pay-to-pubkey script to
	// an uncompressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	CstPayToPubKeyUncomp4 = 4

	// CstPayToPubKeyUncomp5 identifies a compressed pay-to-pubkey script to
	// an uncompressed pubkey.  Bit 0 specifies which y-coordinate to use
	// to reconstruct the full uncompressed pubkey.
	CstPayToPubKeyUncomp5 = 5

	// NumSpecialScripts is the number of special scripts recognized by the
	// domain-specific script compression algorithm.
	NumSpecialScripts = 6
)

func GetScriptSize(scriptType uint64) uint64 {
	switch scriptType {
	case 0, 1:
		return 20
	case 2, 3, 4, 5:
		return 32
	default:
		return scriptType - NumSpecialScripts
	}
}

func DecompressScript(scriptType uint64, compressedPkScript []byte) []byte {
	switch scriptType {
	case CstPayToPubKeyHash:
		pkScript := make([]byte, 25)
		pkScript[0] = txscript.OP_DUP
		pkScript[1] = txscript.OP_HASH160
		pkScript[2] = txscript.OP_DATA_20
		copy(pkScript[3:], compressedPkScript)
		pkScript[23] = txscript.OP_EQUALVERIFY
		pkScript[24] = txscript.OP_CHECKSIG
		return pkScript
	case CstPayToScriptHash:
		pkScript := make([]byte, 23)
		pkScript[0] = txscript.OP_HASH160
		pkScript[1] = txscript.OP_DATA_20
		copy(pkScript[2:], compressedPkScript)
		pkScript[22] = txscript.OP_EQUAL
		return pkScript
	case CstPayToPubKeyComp2, CstPayToPubKeyComp3:
		pkScript := make([]byte, 35)
		pkScript[0] = txscript.OP_DATA_33
		copy(pkScript[1:], compressedPkScript)
		pkScript[34] = txscript.OP_CHECKSIG
		return pkScript
	case CstPayToPubKeyUncomp4, CstPayToPubKeyUncomp5:
		// Change the leading byte to the appropriate compressed pubkey
		// identifier (0x02 or 0x03) so it can be decoded as a
		// compressed pubkey.  This really should never fail since the
		// encoding ensures it is valid before compressing to this type.
		compressedKey := make([]byte, 33)
		compressedKey[0] = byte(scriptType - 2)
		copy(compressedKey[1:], compressedPkScript[1:])
		key, err := btcec.ParsePubKey(compressedKey)
		if err != nil {
			return nil
		}

		pkScript := make([]byte, 67)
		pkScript[0] = txscript.OP_DATA_65
		copy(pkScript[1:], key.SerializeUncompressed())
		pkScript[66] = txscript.OP_CHECKSIG
		return pkScript
	}

	// When none of the special cases apply, the script was encoded using
	// the general format, so reduce the script size by the number of
	// special cases and return the unmodified script.
	return compressedPkScript
}

func CompressTxOutAmount(amount uint64) uint64 {
	// No need to do any work if it's zero.
	if amount == 0 {
		return 0
	}

	// Find the largest power of 10 (max of 9) that evenly divides the
	// value.
	exponent := uint64(0)
	for amount%10 == 0 && exponent < 9 {
		amount /= 10
		exponent++
	}

	// The compressed result for exponents less than 9 is:
	// 1 + 10*(9*n + d-1) + e
	if exponent < 9 {
		lastDigit := amount % 10
		amount /= 10
		return 1 + 10*(9*amount+lastDigit-1) + exponent
	}

	// The compressed result for an exponent of 9 is:
	// 1 + 10*(n-1) + e   ==   10 + 10*(n-1)
	return 10 + 10*(amount-1)
}

// DecompressTxOutAmount returns the original amount the passed compressed
// amount represents according to the domain specific compression algorithm
// described above.
func DecompressTxOutAmount(amount uint64) uint64 {
	// No need to do any work if it's zero.
	if amount == 0 {
		return 0
	}

	// The decompressed amount is either of the following two equations:
	// x = 1 + 10*(9*n + d - 1) + e
	// x = 1 + 10*(n - 1)       + 9
	amount--

	// The decompressed amount is now one of the following two equations:
	// x = 10*(9*n + d - 1) + e
	// x = 10*(n - 1)       + 9
	exponent := amount % 10
	amount /= 10

	// The decompressed amount is now one of the following two equations:
	// x = 9*n + d - 1  | where e < 9
	// x = n - 1        | where e = 9
	n := uint64(0)
	if exponent < 9 {
		lastDigit := amount%9 + 1
		amount /= 9
		n = amount*10 + lastDigit
	} else {
		n = amount + 1
	}

	// Apply the exponent.
	for ; exponent > 0; exponent-- {
		n *= 10
	}

	return n
}

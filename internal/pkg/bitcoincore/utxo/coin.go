package utxo

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"

	"github.com/btcsuite/btcd/wire"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/binaryutils"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bitcoincore/utxocompression"
)

const MAX_SCRIPT_SIZE = 10000

type BytesBuffer interface {
	io.ByteScanner
	io.Reader
}

type Coin struct {
	// Whether the ooutput coinbase or not
	isCoinBase bool
	// On which block height this output was added to UTXO set
	blockHeight uint64

	// Output for the coin
	out *wire.TxOut
}

func NewCoin() *Coin {
	return &Coin{
		out: nil,
	}
}

func (c *Coin) GetOut() *wire.TxOut {
	return c.out
}

func (c *Coin) IsCoinbase() bool {
	return c.isCoinBase
}

func (c *Coin) BlockHeight() uint64 {
	return c.blockHeight
}

func (c *Coin) Deserialize(coinBuffer BytesBuffer) error {
	// Reading code  the ((iscoinbase ? 1 : 0) | blockheight << 1)
	code, _, err := binaryutils.DeserializeVLQ(coinBuffer)
	if err != nil {
		return err
	}

	c.blockHeight = uint64(code >> 1)
	c.isCoinBase = code&1 == 1

	// reading varint amount in satoshi
	amount, _, err := binaryutils.DeserializeVLQ(coinBuffer)
	if err != nil {
		return err
	}

	scriptType, _, err := binaryutils.DeserializeVLQ(coinBuffer)
	if err != nil {
		return fmt.Errorf("failed to read script type: %v", err)
	}

	scriptSize := utxocompression.GetScriptSize(scriptType)

	switch scriptType {
	case utxocompression.CstPayToPubKeyComp2,
		utxocompression.CstPayToPubKeyComp3,
		utxocompression.CstPayToPubKeyUncomp4,
		utxocompression.CstPayToPubKeyUncomp5:
		// we need to return buffer offset to -1 byte
		err := coinBuffer.UnreadByte()
		if err != nil {
			return fmt.Errorf("unread after read scriot size error: %w", err)
		}

		scriptSize++
	}

	if scriptSize > MAX_SCRIPT_SIZE {
		return fmt.Errorf("script size is too large: %d", scriptSize)
	}

	pkScript := make([]byte, scriptSize)
	_, err = coinBuffer.Read(pkScript)
	if err != nil {
		return err
	}

	log.Println("raw amount", amount)
	c.out = wire.NewTxOut(
		int64(amount),
		// int64(utxocompression.DecompressTxOutAmount(amount)),
		utxocompression.DecompressScript(scriptType, pkScript),
	)

	return nil
}

// readUvarint32 reads an unsigned varint from r and returns it as uint32.
func readUvarint32(r io.ByteReader) (uint32, error) {
	// Use binary.ReadUvarint to read the varint as uint64.
	uv, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, err // Propagate errors (e.g., EOF or malformed varint).
	}

	// Check if the read value fits into uint32.
	if uv > 0xFFFFFFFF {
		return 0, fmt.Errorf("value %d overflows uint32", uv)
	}

	// Cast the uint64 to uint32 safely and return.
	return uint32(uv), nil
}

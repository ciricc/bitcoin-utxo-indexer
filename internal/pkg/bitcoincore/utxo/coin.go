package utxo

import (
	"fmt"
	"io"

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

	c.out = wire.NewTxOut(
		int64(utxocompression.DecompressTxOutAmount(amount)),
		utxocompression.DecompressScript(scriptType, pkScript),
	)

	return nil
}

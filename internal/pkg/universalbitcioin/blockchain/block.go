package blockchain

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/bigjson"
)

type Block struct {
	BlockHeader
	Transactions []*Transaction `json:"tx"`
}

func (b *Block) GetTransactions() []*Transaction {
	return b.Transactions
}

// BlockHeader is a structure for storing the block header data
// It is used for getting the block header from the blockchain
// This realisation is most compatible with bitcoin blockchain
type BlockHeader struct {
	Hash          Hash           `json:"hash"`
	NextBlockHash Hash           `json:"nextblockhash"`
	PrevBlockHash Hash           `json:"previousblockhash"`
	Height        int64          `json:"height"`
	MerkleRoot    string         `json:"merkleroot"`
	Nonce         Nonce          `json:"nonce"`
	Nonce64       bigjson.BigInt `json:"nonce64"`
	Size          int64          `json:"size"`
	Time          int64          `json:"time"`
	Version       int            `json:"version"`
	Weight        int64          `json:"weight"`
	Difficulty    float64        `json:"difficulty"`
	Bits          string         `json:"bits"`
}

func (b *BlockHeader) Valid() bool {
	return len(b.Hash) != 0
}

func (b *BlockHeader) GetHash() Hash {
	return b.Hash
}

func (b *BlockHeader) GetNextBlockHash() Hash {
	return b.NextBlockHash
}

func (b *BlockHeader) GetPrevBlockHash() Hash {
	return b.PrevBlockHash
}

func (b *BlockHeader) GetHeight() int64 {
	return b.Height
}

func (b *BlockHeader) GetMerkleRoot() string {
	return b.MerkleRoot
}

func (b *BlockHeader) GetNonce() string {
	if b.Nonce64.Cmp(big.NewInt(0)) != 0 {
		return b.Nonce64.String()
	}

	return b.Nonce.String()
}

func (b *BlockHeader) GetSize() int64 {
	return b.Size
}

func (b *BlockHeader) GetTime() int64 {
	return b.Time
}

func (b *BlockHeader) GetVersion() int {
	return b.Version
}

func (b *BlockHeader) GetWeight() int64 {
	return b.Weight
}

func (b *BlockHeader) GetDifficulty() float64 {
	return b.Difficulty
}

func (b *BlockHeader) GetBits() string {
	return b.Bits
}

// Nonce is a string of numeric value or hexademical
// It stores in the transaction for preventing the duplication
// of it and for maintain the right order of the transactions history
// in the blockchain
//
// In the bitcoin nonce is a unit64 value but in another
// Bitcoin-compatible networks nonce may be represented as a non-numeric value
// This Nonce realisation is most compatible with bitcoin
type Nonce struct {
	parsedNonce string
}

func (n *Nonce) String() string {
	return n.parsedNonce
}

func (n *Nonce) MarshalJSON() ([]byte, error) {
	if n == nil {
		return []byte(`""`), nil
	}

	return json.Marshal(n.String())
}

func (n *Nonce) UnmarshalJSON(p []byte) error {
	var nonceString string

	err := json.Unmarshal(p, &nonceString)
	if err == nil {
		n.parsedNonce = nonceString
		return nil
	}

	var nonceInt64 bigjson.BigInt
	if err := json.Unmarshal(p, &nonceInt64); err == nil {
		n.parsedNonce = nonceInt64.String()
		return nil
	}

	return fmt.Errorf("failed to unmarshal nonce: %s", string(p))
}

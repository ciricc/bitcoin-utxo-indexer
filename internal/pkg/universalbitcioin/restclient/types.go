package restclient

import "github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"

type Hash interface {
	String() string
}

type GetBlockHashByHeightResponse struct {
	BlockHash blockchain.Hash `json:"blockhash"`
}

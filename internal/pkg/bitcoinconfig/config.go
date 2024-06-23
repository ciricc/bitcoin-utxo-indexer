package bitcoinconfig

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/universalbitcioin/blockchain"
)

type BitcoinConfig struct {
	params   *chaincfg.Params
	decimals int
}

type BitcoinRESTClient interface {
	GetBlockchainInfo(ctx context.Context) (*blockchain.BlockchainInfo, error)
}

func New(
	btcClient BitcoinRESTClient,
	decimals int,
) (*BitcoinConfig, error) {
	bcInfo, err := btcClient.GetBlockchainInfo(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get blockchain info: %w", err)
	}

	var useParams *chaincfg.Params

	mainParams := chaincfg.MainNetParams
	mainParams.Name = "main"

	params := []*chaincfg.Params{
		&mainParams,
		&chaincfg.MainNetParams,
		&chaincfg.TestNet3Params,
		&chaincfg.RegressionNetParams,
		&chaincfg.SimNetParams,
	}

	for _, p := range params {
		if p.Name == bcInfo.Chain {
			useParams = p
			break
		}
	}

	if useParams == nil {
		return nil, fmt.Errorf("unknown chain: %s", bcInfo.Chain)
	}

	return &BitcoinConfig{
		params:   useParams,
		decimals: decimals,
	}, nil
}

func (bc *BitcoinConfig) GetParams() *chaincfg.Params {
	return bc.params
}

func (bc *BitcoinConfig) GetDecimals() int {
	return bc.decimals
}

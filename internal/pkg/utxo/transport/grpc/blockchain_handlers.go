package grpchandlers

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/pkg/api/grpc/TxOuts_V1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type UTXOBlockchainService interface {
	// GetBlockHeight must return the block height
	GetBlockHeight(ctx context.Context) (int64, error)

	// GetBlockHash must return the block hash
	GetBlockHash(ctx context.Context) (string, error)
}

type BlockchainInfo interface {
	// GetBlocksCount must return blocks count of the blockchain node
	GetBlocksCount(ctx context.Context) (int64, error)
	// GetHeadersCoutnm ust return headers count of the blockchain node
	GetHeadersCount(ctx context.Context) (int64, error)
}

type TxOutsV1BlockchainHandlers struct {
	TxOuts_V1.UnimplementedBlockchainServer

	s              UTXOBlockchainService
	blockchainInfo BlockchainInfo
	btcConfig      BitcoinConfig
}

func NewV1BlockchainHandlers(
	s UTXOBlockchainService,
	btcConfig BitcoinConfig,
	blockchainInfo BlockchainInfo,
) *TxOutsV1BlockchainHandlers {
	return &TxOutsV1BlockchainHandlers{
		UnimplementedBlockchainServer: TxOuts_V1.UnimplementedBlockchainServer{},
		s:                             s,
		btcConfig:                     btcConfig,
		blockchainInfo:                blockchainInfo,
	}
}

func (t *TxOutsV1BlockchainHandlers) GetInfo(
	ctx context.Context,
	req *emptypb.Empty,
) (*TxOuts_V1.BlockchainInfo, error) {
	blockHeight, err := t.s.GetBlockHeight(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get blockchain height")
	}

	blockHash, err := t.s.GetBlockHash(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed toget blockchain hash")
	}

	nodeBlocksCount, err := t.blockchainInfo.GetBlocksCount(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get node's blocks count")
	}

	nodeHeadersCount, err := t.blockchainInfo.GetHeadersCount(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get node's headers count")
	}

	nodeIsSynced := nodeHeadersCount == nodeBlocksCount

	return &TxOuts_V1.BlockchainInfo{
		Height:        blockHeight,
		BestBlockHash: blockHash,
		Decimals:      int32(t.btcConfig.GetDecimals()),
		IsSynced:      nodeBlocksCount == blockHeight && nodeIsSynced,
	}, nil
}

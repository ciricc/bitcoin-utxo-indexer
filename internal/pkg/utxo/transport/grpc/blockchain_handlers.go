package grpchandlers

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/pkg/api/grpc/TxOuts_V1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BlockchainService interface {
	// GetBlockHeight must return the block height
	GetBlockHeight(ctx context.Context) (int64, error)
}

type TxOutsV1BlockchainHandlers struct {
	TxOuts_V1.UnimplementedBlockchainServer

	s BlockchainService
}

func NewV1BlockchainHandlers(
	s BlockchainService,
) *TxOutsV1BlockchainHandlers {
	return &TxOutsV1BlockchainHandlers{
		UnimplementedBlockchainServer: TxOuts_V1.UnimplementedBlockchainServer{},
		s:                             s,
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

	return &TxOuts_V1.BlockchainInfo{
		Height: blockHeight,
	}, nil
}

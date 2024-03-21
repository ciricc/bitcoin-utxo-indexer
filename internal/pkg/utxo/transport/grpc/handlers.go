package grpchandlers

import (
	"context"

	"github.com/ciricc/btc-utxo-indexer/pkg/api/grpc/UTXO"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UTXOService interface {
	// GetUTXOByAdress must return list of UTXO of the address
	GetUTXOByAddress(ctx context.Context, address string) ([]bool, error)
}

type UTXOGrpcHandlers struct {
	UTXO.UnimplementedUTXOServer

	s UTXOService
}

func New(service UTXOService) *UTXOGrpcHandlers {
	return &UTXOGrpcHandlers{
		UnimplementedUTXOServer: UTXO.UnimplementedUTXOServer{},
		s:                       service,
	}
}

func (u *UTXOGrpcHandlers) GetByAddress(
	ctx context.Context,
	req *UTXO.GetByAddressRequest,
) (*UTXO.GetByAddressResponse, error) {
	outputs, err := u.s.GetUTXOByAddress(ctx, req.Address)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to get UTXO",
		)
	}

	m := make([]*UTXO.UnspentTransactionOutput, 0, len(outputs))

	for range outputs {
		m = append(m, &UTXO.UnspentTransactionOutput{})
	}

	return &UTXO.GetByAddressResponse{
		UnspentOutputs: m,
	}, nil
}

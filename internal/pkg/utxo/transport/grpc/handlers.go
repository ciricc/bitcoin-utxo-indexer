package grpchandlers

import (
	"context"
	"encoding/hex"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/ciricc/btc-utxo-indexer/pkg/api/grpc/UTXO"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type UTXOService interface {
	// GetUTXOByBase58Adress must return list of UTXO of the address
	GetUTXOByBase58Address(ctx context.Context, address string) ([]*utxostore.UTXOEntry, error)
	GetBlockHeight(ctx context.Context) (int64, error)
}

type BitcoinConfig interface {
	GetDecimals() int
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

func (u *UTXOGrpcHandlers) GetBlockHeight(
	ctx context.Context,
	_ *emptypb.Empty,
) (*UTXO.GetBlockHeightResponse, error) {
	height, err := u.s.GetBlockHeight(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &UTXO.GetBlockHeightResponse{
		BlockHeight: height,
	}, nil
}

func (u *UTXOGrpcHandlers) TotalAmountByAddress(
	ctx context.Context,
	address *UTXO.Address,
) (*UTXO.Amount, error) {
	outputs, err := u.s.GetUTXOByBase58Address(ctx, address.Base58)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get outputs")
	}

	var totalAmount uint64
	for _, output := range outputs {
		totalAmount += output.Output.GetAmount()
	}

	return &UTXO.Amount{
		Value: totalAmount,
	}, nil
}

func (u *UTXOGrpcHandlers) ListByAddress(
	ctx context.Context,
	address *UTXO.Address,
) (*UTXO.TransactionOutputs, error) {
	outputs, err := u.s.GetUTXOByBase58Address(ctx, address.Base58)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to get UTXO",
		)
	}

	m := make([]*UTXO.TransactionOutput, 0, len(outputs))

	for _, output := range outputs {
		scriptBytes := output.Output.GetScriptBytes()
		m = append(m, &UTXO.TransactionOutput{
			TxId: output.TxID,
			Amount: &UTXO.Amount{
				Value: output.Output.GetAmount(),
			},
			ScriptPubKey: hex.EncodeToString(scriptBytes),
			Index:        int32(output.Vout),
		})
	}

	return &UTXO.TransactionOutputs{
		Items: m,
	}, nil
}

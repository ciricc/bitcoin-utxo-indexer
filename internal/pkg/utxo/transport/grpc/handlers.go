package grpchandlers

import (
	"context"
	"encoding/hex"

	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/ciricc/btc-utxo-indexer/pkg/api/grpc/UTXO"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	s             UTXOService
	bitcoinConfig BitcoinConfig
}

func New(service UTXOService, btcConfig BitcoinConfig) *UTXOGrpcHandlers {
	return &UTXOGrpcHandlers{
		UnimplementedUTXOServer: UTXO.UnimplementedUTXOServer{},
		s:                       service,
		bitcoinConfig:           btcConfig,
	}
}

func (u *UTXOGrpcHandlers) GetBlockHeight(
	ctx context.Context,
	_ *UTXO.GetBlockHeightRequest,
) (*UTXO.GetBlockHeightResponse, error) {
	height, err := u.s.GetBlockHeight(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &UTXO.GetBlockHeightResponse{
		BlockHeight: height,
	}, nil
}

func (u *UTXOGrpcHandlers) GetByAddress(
	ctx context.Context,
	req *UTXO.GetByAddressRequest,
) (*UTXO.GetByAddressResponse, error) {
	outputs, err := u.s.GetUTXOByBase58Address(ctx, req.Address)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"failed to get UTXO",
		)
	}

	m := make([]*UTXO.UnspentTransactionOutput, 0, len(outputs))

	for _, output := range outputs {
		scriptBytes := output.Output.GetScriptBytes()
		amount, err := output.Output.GetAmountFloat64(u.bitcoinConfig.GetDecimals())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get utxo amount")
		}

		m = append(m, &UTXO.UnspentTransactionOutput{
			TxId:         output.TxID,
			Amount:       amount.String(),
			ScriptPubKey: hex.EncodeToString(scriptBytes),
			Index:        int32(output.Vout),
		})
	}

	return &UTXO.GetByAddressResponse{
		UnspentOutputs: m,
	}, nil
}

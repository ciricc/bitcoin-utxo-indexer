package grpchandlers

import (
	"context"
	"encoding/hex"
	"errors"

	utxoservice "github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/service"
	"github.com/ciricc/btc-utxo-indexer/internal/pkg/utxo/utxostore"
	"github.com/ciricc/btc-utxo-indexer/pkg/api/grpc/TxOuts_V1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UTXOService interface {
	// GetUTXOByBase58Adress must return list of UTXO of the address
	GetUTXOByBase58Address(ctx context.Context, address string) ([]*utxostore.UTXOEntry, error)
}

type BitcoinConfig interface {
	GetDecimals() int
}

type TxOutsV1AddressesHandlers struct {
	TxOuts_V1.UnimplementedAddressesServer

	s UTXOService
}

func NewV1AddressHandlers(service UTXOService) *TxOutsV1AddressesHandlers {
	return &TxOutsV1AddressesHandlers{
		UnimplementedAddressesServer: TxOuts_V1.UnimplementedAddressesServer{},

		s: service,
	}
}

func (u *TxOutsV1AddressesHandlers) GetBalance(
	ctx context.Context,
	address *TxOuts_V1.EncodedAddress,
) (*TxOuts_V1.Amount, error) {
	outputs, err := u.s.GetUTXOByBase58Address(ctx, address.Value)
	if err != nil {
		if errors.Is(err, utxoservice.ErrInvalidBase58Address) {
			return nil, status.Error(codes.InvalidArgument, "invalid address")
		}

		return nil, status.Error(codes.Internal, "failed to get outputs")
	}

	var totalAmount uint64
	for _, output := range outputs {
		totalAmount += output.Output.GetAmount()
	}

	return &TxOuts_V1.Amount{
		Value: totalAmount,
	}, nil
}

func (u *TxOutsV1AddressesHandlers) GetUnspentOutputs(
	ctx context.Context,
	address *TxOuts_V1.EncodedAddress,
) (*TxOuts_V1.TransactionOutputs, error) {
	outputs, err := u.s.GetUTXOByBase58Address(ctx, address.Value)
	if err != nil {
		if errors.Is(err, utxoservice.ErrInvalidBase58Address) {
			return nil, status.Error(codes.InvalidArgument, "invalid address")
		}

		return nil, status.Errorf(
			codes.Internal,
			"failed to get UTXO",
		)
	}

	m := make([]*TxOuts_V1.TransactionOutput, 0, len(outputs))

	for _, output := range outputs {
		scriptBytes := output.Output.GetScriptBytes()
		m = append(m, &TxOuts_V1.TransactionOutput{
			TxId: output.TxID,
			Amount: &TxOuts_V1.Amount{
				Value: output.Output.GetAmount(),
			},
			ScriptPubKey: hex.EncodeToString(scriptBytes),
			Index:        int32(output.Vout),
		})
	}

	return &TxOuts_V1.TransactionOutputs{
		Items: m,
	}, nil
}

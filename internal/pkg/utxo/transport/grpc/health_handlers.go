package grpchandlers

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type HealthGRPCHandlers struct {
	grpc_health_v1.UnimplementedHealthServer

	blockchainInfo        BlockchainInfo
	utxoBlockchainService UTXOBlockchainService
}

type InMemoryHealthClient struct {
	h *HealthGRPCHandlers
}

func (i *InMemoryHealthClient) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest, opts ...grpc.CallOption) (
	*grpc_health_v1.HealthCheckResponse, error,
) {
	return i.h.Check(ctx, req)
}

func (i *InMemoryHealthClient) Watch(ctx context.Context, in *grpc_health_v1.HealthCheckRequest, opts ...grpc.CallOption) (grpc_health_v1.Health_WatchClient, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func NewHealthClient(handlers *HealthGRPCHandlers) *InMemoryHealthClient {
	return &InMemoryHealthClient{
		h: handlers,
	}
}

func NewHealthHandlers(
	blockchainInfo BlockchainInfo,
	utxoBlockchainService UTXOBlockchainService,
) (*HealthGRPCHandlers, error) {
	return &HealthGRPCHandlers{
		UnimplementedHealthServer: grpc_health_v1.UnimplementedHealthServer{},

		blockchainInfo:        blockchainInfo,
		utxoBlockchainService: utxoBlockchainService,
	}, nil
}

func (h *HealthGRPCHandlers) Check(ctx context.Context, in *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	blockchainBlocksCount, err := h.blockchainInfo.GetBlocksCount(ctx)
	if err != nil {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	indexerBlocksCount, err := h.utxoBlockchainService.GetBlockHeight(ctx)
	if err != nil {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	if blockchainBlocksCount != indexerBlocksCount {
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

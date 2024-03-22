// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.12.4
// source: api/grpc/proto/utxo.proto

package UTXO

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	UTXO_GetByAddress_FullMethodName   = "/UTXO/GetByAddress"
	UTXO_GetBlockHeight_FullMethodName = "/UTXO/GetBlockHeight"
)

// UTXOClient is the client API for UTXO service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type UTXOClient interface {
	// GetByAddress returns list of UTXO by address
	GetByAddress(ctx context.Context, in *GetByAddressRequest, opts ...grpc.CallOption) (*GetByAddressResponse, error)
	// GetBlockHeight returns the number of the block last synced with in UTXO storage
	GetBlockHeight(ctx context.Context, in *GetBlockHeightRequest, opts ...grpc.CallOption) (*GetBlockHeightResponse, error)
}

type uTXOClient struct {
	cc grpc.ClientConnInterface
}

func NewUTXOClient(cc grpc.ClientConnInterface) UTXOClient {
	return &uTXOClient{cc}
}

func (c *uTXOClient) GetByAddress(ctx context.Context, in *GetByAddressRequest, opts ...grpc.CallOption) (*GetByAddressResponse, error) {
	out := new(GetByAddressResponse)
	err := c.cc.Invoke(ctx, UTXO_GetByAddress_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *uTXOClient) GetBlockHeight(ctx context.Context, in *GetBlockHeightRequest, opts ...grpc.CallOption) (*GetBlockHeightResponse, error) {
	out := new(GetBlockHeightResponse)
	err := c.cc.Invoke(ctx, UTXO_GetBlockHeight_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UTXOServer is the server API for UTXO service.
// All implementations must embed UnimplementedUTXOServer
// for forward compatibility
type UTXOServer interface {
	// GetByAddress returns list of UTXO by address
	GetByAddress(context.Context, *GetByAddressRequest) (*GetByAddressResponse, error)
	// GetBlockHeight returns the number of the block last synced with in UTXO storage
	GetBlockHeight(context.Context, *GetBlockHeightRequest) (*GetBlockHeightResponse, error)
	mustEmbedUnimplementedUTXOServer()
}

// UnimplementedUTXOServer must be embedded to have forward compatible implementations.
type UnimplementedUTXOServer struct {
}

func (UnimplementedUTXOServer) GetByAddress(context.Context, *GetByAddressRequest) (*GetByAddressResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetByAddress not implemented")
}
func (UnimplementedUTXOServer) GetBlockHeight(context.Context, *GetBlockHeightRequest) (*GetBlockHeightResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockHeight not implemented")
}
func (UnimplementedUTXOServer) mustEmbedUnimplementedUTXOServer() {}

// UnsafeUTXOServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to UTXOServer will
// result in compilation errors.
type UnsafeUTXOServer interface {
	mustEmbedUnimplementedUTXOServer()
}

func RegisterUTXOServer(s grpc.ServiceRegistrar, srv UTXOServer) {
	s.RegisterService(&UTXO_ServiceDesc, srv)
}

func _UTXO_GetByAddress_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetByAddressRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UTXOServer).GetByAddress(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UTXO_GetByAddress_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UTXOServer).GetByAddress(ctx, req.(*GetByAddressRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _UTXO_GetBlockHeight_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetBlockHeightRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UTXOServer).GetBlockHeight(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UTXO_GetBlockHeight_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UTXOServer).GetBlockHeight(ctx, req.(*GetBlockHeightRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// UTXO_ServiceDesc is the grpc.ServiceDesc for UTXO service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var UTXO_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "UTXO",
	HandlerType: (*UTXOServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetByAddress",
			Handler:    _UTXO_GetByAddress_Handler,
		},
		{
			MethodName: "GetBlockHeight",
			Handler:    _UTXO_GetBlockHeight_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/grpc/proto/utxo.proto",
}

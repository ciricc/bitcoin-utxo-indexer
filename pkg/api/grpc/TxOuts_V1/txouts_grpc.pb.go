// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.12.4
// source: txouts.proto

package TxOuts_V1

import (
	context "context"
	empty "github.com/golang/protobuf/ptypes/empty"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Blockchain_GetInfo_FullMethodName = "/TxOuts.V1.Blockchain/GetInfo"
)

// BlockchainClient is the client API for Blockchain service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BlockchainClient interface {
	// Returns information about the blockchain
	//
	// It doesn't return information about the node's blockchain,
	// because the service syncing with it independently
	// So, the blockchain may not be synced with the bitcoin node
	GetInfo(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*BlockchainInfo, error)
}

type blockchainClient struct {
	cc grpc.ClientConnInterface
}

func NewBlockchainClient(cc grpc.ClientConnInterface) BlockchainClient {
	return &blockchainClient{cc}
}

func (c *blockchainClient) GetInfo(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*BlockchainInfo, error) {
	out := new(BlockchainInfo)
	err := c.cc.Invoke(ctx, Blockchain_GetInfo_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BlockchainServer is the server API for Blockchain service.
// All implementations must embed UnimplementedBlockchainServer
// for forward compatibility
type BlockchainServer interface {
	// Returns information about the blockchain
	//
	// It doesn't return information about the node's blockchain,
	// because the service syncing with it independently
	// So, the blockchain may not be synced with the bitcoin node
	GetInfo(context.Context, *empty.Empty) (*BlockchainInfo, error)
	mustEmbedUnimplementedBlockchainServer()
}

// UnimplementedBlockchainServer must be embedded to have forward compatible implementations.
type UnimplementedBlockchainServer struct {
}

func (UnimplementedBlockchainServer) GetInfo(context.Context, *empty.Empty) (*BlockchainInfo, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetInfo not implemented")
}
func (UnimplementedBlockchainServer) mustEmbedUnimplementedBlockchainServer() {}

// UnsafeBlockchainServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BlockchainServer will
// result in compilation errors.
type UnsafeBlockchainServer interface {
	mustEmbedUnimplementedBlockchainServer()
}

func RegisterBlockchainServer(s grpc.ServiceRegistrar, srv BlockchainServer) {
	s.RegisterService(&Blockchain_ServiceDesc, srv)
}

func _Blockchain_GetInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BlockchainServer).GetInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Blockchain_GetInfo_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BlockchainServer).GetInfo(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// Blockchain_ServiceDesc is the grpc.ServiceDesc for Blockchain service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Blockchain_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "TxOuts.V1.Blockchain",
	HandlerType: (*BlockchainServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetInfo",
			Handler:    _Blockchain_GetInfo_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "txouts.proto",
}

const (
	Addresses_GetBalance_FullMethodName        = "/TxOuts.V1.Addresses/GetBalance"
	Addresses_GetUnspentOutputs_FullMethodName = "/TxOuts.V1.Addresses/GetUnspentOutputs"
)

// AddressesClient is the client API for Addresses service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type AddressesClient interface {
	// Returns balance in satoshi of the account by its encoded address
	//
	// If the address is valid but not existing in the blockchain,
	// method will return the zero balance amount.
	//
	// Errors:
	// INVALID_ARGMUENT - if the address has invalid format
	GetBalance(ctx context.Context, in *EncodedAddress, opts ...grpc.CallOption) (*AccountBalance, error)
	// Returns the list of the unspent outputs by encoded address
	//
	// If there is no unspent outputs by the valid address, it returns empty list.
	//
	// Errors:
	// INVALID_ARGUMENT - if the address has invalid format
	GetUnspentOutputs(ctx context.Context, in *EncodedAddress, opts ...grpc.CallOption) (*TransactionOutputs, error)
}

type addressesClient struct {
	cc grpc.ClientConnInterface
}

func NewAddressesClient(cc grpc.ClientConnInterface) AddressesClient {
	return &addressesClient{cc}
}

func (c *addressesClient) GetBalance(ctx context.Context, in *EncodedAddress, opts ...grpc.CallOption) (*AccountBalance, error) {
	out := new(AccountBalance)
	err := c.cc.Invoke(ctx, Addresses_GetBalance_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *addressesClient) GetUnspentOutputs(ctx context.Context, in *EncodedAddress, opts ...grpc.CallOption) (*TransactionOutputs, error) {
	out := new(TransactionOutputs)
	err := c.cc.Invoke(ctx, Addresses_GetUnspentOutputs_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AddressesServer is the server API for Addresses service.
// All implementations must embed UnimplementedAddressesServer
// for forward compatibility
type AddressesServer interface {
	// Returns balance in satoshi of the account by its encoded address
	//
	// If the address is valid but not existing in the blockchain,
	// method will return the zero balance amount.
	//
	// Errors:
	// INVALID_ARGMUENT - if the address has invalid format
	GetBalance(context.Context, *EncodedAddress) (*AccountBalance, error)
	// Returns the list of the unspent outputs by encoded address
	//
	// If there is no unspent outputs by the valid address, it returns empty list.
	//
	// Errors:
	// INVALID_ARGUMENT - if the address has invalid format
	GetUnspentOutputs(context.Context, *EncodedAddress) (*TransactionOutputs, error)
	mustEmbedUnimplementedAddressesServer()
}

// UnimplementedAddressesServer must be embedded to have forward compatible implementations.
type UnimplementedAddressesServer struct {
}

func (UnimplementedAddressesServer) GetBalance(context.Context, *EncodedAddress) (*AccountBalance, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetBalance not implemented")
}
func (UnimplementedAddressesServer) GetUnspentOutputs(context.Context, *EncodedAddress) (*TransactionOutputs, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUnspentOutputs not implemented")
}
func (UnimplementedAddressesServer) mustEmbedUnimplementedAddressesServer() {}

// UnsafeAddressesServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to AddressesServer will
// result in compilation errors.
type UnsafeAddressesServer interface {
	mustEmbedUnimplementedAddressesServer()
}

func RegisterAddressesServer(s grpc.ServiceRegistrar, srv AddressesServer) {
	s.RegisterService(&Addresses_ServiceDesc, srv)
}

func _Addresses_GetBalance_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EncodedAddress)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AddressesServer).GetBalance(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Addresses_GetBalance_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AddressesServer).GetBalance(ctx, req.(*EncodedAddress))
	}
	return interceptor(ctx, in, info, handler)
}

func _Addresses_GetUnspentOutputs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EncodedAddress)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AddressesServer).GetUnspentOutputs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Addresses_GetUnspentOutputs_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AddressesServer).GetUnspentOutputs(ctx, req.(*EncodedAddress))
	}
	return interceptor(ctx, in, info, handler)
}

// Addresses_ServiceDesc is the grpc.ServiceDesc for Addresses service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Addresses_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "TxOuts.V1.Addresses",
	HandlerType: (*AddressesServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetBalance",
			Handler:    _Addresses_GetBalance_Handler,
		},
		{
			MethodName: "GetUnspentOutputs",
			Handler:    _Addresses_GetUnspentOutputs_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "txouts.proto",
}

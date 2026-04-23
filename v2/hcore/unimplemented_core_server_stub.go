//go:build !legacy_grpc_runtime
// +build !legacy_grpc_runtime

package hcore

// UnimplementedCoreServer keeps the default build free from generated gRPC glue.
type UnimplementedCoreServer struct{}

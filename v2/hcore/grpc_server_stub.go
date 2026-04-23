package hcore

import "fmt"

type grpcServerHandle struct{}

var grpcServer map[SetupMode]*grpcServerHandle = make(map[SetupMode]*grpcServerHandle)

func StartGrpcServer(listenAddressG string, service string) (*grpcServerHandle, error) {
	_ = listenAddressG
	_ = service
	return nil, fmt.Errorf("legacy gRPC runtime is disabled in this build")
}

func StartCoreGrpcServer(listenAddressG string) (*grpcServerHandle, error) {
	_ = listenAddressG
	return nil, fmt.Errorf("legacy gRPC runtime is disabled in this build")
}

func StartHelloGrpcServer(listenAddressG string) (*grpcServerHandle, error) {
	_ = listenAddressG
	return nil, fmt.Errorf("legacy gRPC runtime is disabled in this build")
}

func StartGrpcServerByMode(listenAddressG string, mode SetupMode) (*grpcServerHandle, error) {
	_ = listenAddressG
	_ = mode
	return nil, fmt.Errorf("legacy gRPC runtime is disabled in this build")
}

func GetGrpcServerPublicKey() []byte {
	return nil
}

func AddGrpcClientPublicKey(clientPublicKey []byte) error {
	_ = clientPublicKey
	return nil
}

func CloseGrpcServer(mode SetupMode) {
	delete(grpcServer, mode)
}

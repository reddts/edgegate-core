package hcore

import "github.com/sagernet/sing-box/experimental/libbox"

// CoreRPCServer implements the gRPC Core service handlers.
type CoreRPCServer struct {
	UnimplementedCoreServer
}

// InstanceService wraps a running libbox instance and its runtime metadata.
type InstanceService struct {
	libbox     *libbox.BoxService
	ListenPort uint16
}

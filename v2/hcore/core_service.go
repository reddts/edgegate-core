package hcore

// CoreRPCServer implements the gRPC Core service handlers.
type CoreRPCServer struct {
	UnimplementedCoreServer
}

// InstanceService wraps a running libbox instance and its runtime metadata.
type InstanceService struct {
	libbox     *BoxService
	ListenPort uint16
}

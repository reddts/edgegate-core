package tunnelservice

import "fmt"

func ActivateTunnelService(opt *TunnelStartRequest) error {
	_ = opt
	return fmt.Errorf("legacy gRPC tunnel service is disabled in this build")
}

func DeactivateTunnelServiceForce() error {
	return nil
}

func DeactivateTunnelService() error {
	return nil
}

func ExitTunnelService() (bool, error) {
	return false, nil
}

func StartTunnelService(goArg string) (int, string) {
	_ = goArg
	return 1, "legacy gRPC tunnel service is disabled in this build"
}

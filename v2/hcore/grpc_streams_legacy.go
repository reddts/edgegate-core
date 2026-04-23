//go:build legacy_grpc_runtime
// +build legacy_grpc_runtime

package hcore

import (
	"time"

	hcommon "github.com/reddts/edgegate-core/v2/hcommon"
)

func (s *CoreRPCServer) CoreInfoListener(req *hcommon.Empty, stream Core_CoreInfoListenerServer) error {
	coreSub, done, err := static.coreInfoObserver.Subscribe()
	if err != nil {
		return err
	}
	defer static.coreInfoObserver.UnSubscribe(coreSub)
	stream.Send(&CoreInfoResponse{
		CoreState:   static.CoreState,
		MessageType: MessageType_EMPTY,
		Message:     "",
	})
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-done:
			return nil
		case info := <-coreSub:
			stream.Send(info)
		}
	}
}

func (s *CoreRPCServer) GetSystemInfo(req *hcommon.Empty, stream Core_GetSystemInfoServer) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	currentStatus := readStatus(nil)
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			currentStatus = readStatus(currentStatus)
			stream.Send(currentStatus)
		}
	}
}

func (s *CoreRPCServer) OutboundsInfo(req *hcommon.Empty, stream Core_OutboundsInfoServer) error {
	return allProxiesInfoStream(stream, false)
}

func (s *CoreRPCServer) MainOutboundsInfo(req *hcommon.Empty, stream Core_MainOutboundsInfoServer) error {
	return allProxiesInfoStream(stream, true)
}

func (s *CoreRPCServer) LogListener(req *hcommon.Empty, stream Core_LogListenerServer) error {
	logSub, stopch, _ := static.logObserver.Subscribe()
	defer static.logObserver.UnSubscribe(logSub)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-stopch:
			return nil
		case info := <-logSub:
			stream.Send(info)
		}
	}
}

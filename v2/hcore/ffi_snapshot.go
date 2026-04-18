package hcore

import "sync"

var (
	ffiStatusLock sync.Mutex
	ffiStatusPrev *SystemInfo
)

// SnapshotCoreInfo returns a lightweight core status snapshot for FFI callers.
func SnapshotCoreInfo() *CoreInfoResponse {
	return &CoreInfoResponse{
		CoreState:   static.CoreState,
		MessageType: MessageType_EMPTY,
		Message:     "",
	}
}

// SnapshotSystemInfo returns a point-in-time system info object.
func SnapshotSystemInfo() *SystemInfo {
	ffiStatusLock.Lock()
	defer ffiStatusLock.Unlock()
	ffiStatusPrev = readStatus(ffiStatusPrev)
	return ffiStatusPrev
}

// SnapshotOutbounds returns outbound groups; onlyMain mirrors MainOutboundsInfo behavior.
func SnapshotOutbounds(onlyMain bool) *OutboundGroupList {
	return GetAllProxiesInfo(onlyMain)
}

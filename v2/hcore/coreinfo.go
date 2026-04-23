package hcore

import (
	"fmt"
)

func SetCoreStatus(state CoreStates, msgType MessageType, message string) *CoreInfoResponse {
	msg := fmt.Sprintf("%s: %s %s", state.String(), msgType.String(), message)
	if msgType == MessageType_EMPTY {
		msg = fmt.Sprintf("%s: %s", state.String(), message)
	}
	Log(LogLevel_INFO, LogType_CORE, msg)
	static.CoreState = state
	info := CoreInfoResponse{
		CoreState:   state,
		MessageType: msgType,
		Message:     message,
	}
	static.coreInfoObserver.Emit(&info)

	return &info
}

//go:build legacy_grpc_runtime
// +build legacy_grpc_runtime

package main

/*
#include <stdlib.h>
#include "stdint.h"
*/
import "C"

import (
	hcore "github.com/reddts/edgegate-core/v2/hcore"
	"unsafe"
)

//export StartCoreGrpcServer
func StartCoreGrpcServer(listenAddress *C.char) (CErr *C.char) {
	_, err := hcore.StartCoreGrpcServer(C.GoString(listenAddress))
	return emptyOrErrorC(err)
}

//export GetServerPublicKey
func GetServerPublicKey() *C.char {
	publicKey := hcore.GetGrpcServerPublicKey()
	return C.CString(string(publicKey))
}

//export AddGrpcClientPublicKey
func AddGrpcClientPublicKey(clientPublicKey *C.char) *C.char {
	clientKey := C.GoBytes(unsafe.Pointer(clientPublicKey), C.int(len(C.GoString(clientPublicKey))))
	err := hcore.AddGrpcClientPublicKey(clientKey)
	return emptyOrErrorC(err)
}

//export closeGrpc
func closeGrpc(mode C.int) {
	hcore.Close(hcore.SetupMode(mode))
}

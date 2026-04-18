package main

/*
#include <stdlib.h>
#include <signal.h>
#include "stdint.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	// "os"
	// "os/signal"
	"os"
	"runtime"
	// "syscall"
	"unsafe"

	hcore "github.com/reddts/edgegate-core/v2/hcore"
	"github.com/sagernet/sing-box/log"
)

// func init() {
// 	runtime.LockOSThread()
// 	C.init_signals()
// 	runtime.UnlockOSThread()

// 	go handleSignals()

// 	// Your other initialization code can go here
// }

// // Signal handling function
// func handleSignals() {
// 	signalChan := make(chan os.Signal, 1)
// 	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGURG)

// 	for {
// 		<-signalChan
// 		// switch sig {
// 		// case syscall.SIGINT, syscall.SIGTERM:
// 		// 	// runtime.LockOSThread() // Lock to the current OS thread
// 		// 	// defer runtime.UnlockOSThread()
// 		// 	log.Info("Received signal:", sig)

// 		// 	// Call stop function or perform cleanup
// 		// 	if err := stop(); err != nil {
// 		// 		log.Error("Error stopping the application:", err)
// 		// 	}
// 		// 	log.Info("Application stopped gracefully.")
// 		// }
// 	}
// }

func main() {}

//export cleanup
func cleanup() {
	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()
	// C.cleanup_signals()
}

func emptyOrErrorC(err error) *C.char {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err == nil {
		return C.CString("")
	}
	log.Error(err.Error())
	str := C.CString(err.Error())
	defer C.free(unsafe.Pointer(str))
	return str
}

func jsonResult(ok bool, errMsg string, data any) *C.char {
	result := map[string]any{
		"ok": ok,
	}
	if errMsg != "" {
		result["error"] = errMsg
	}
	if data != nil {
		result["data"] = data
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return C.CString(`{"ok":false,"error":"marshal result failed"}`)
	}
	return C.CString(string(raw))
}

//export setup
func setup(baseDir *C.char, workingDir *C.char, tempDir *C.char, mode C.int, listen *C.char, secret *C.char, statusPort C.longlong, debug bool) *C.char {
	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

	// // Ensure signals are initialized
	// C.init_signals()

	// 若调用方未显式传入 gRPC 监听地址，则使用环境变量或默认端口，避免客户端端口不一致导致连接被拒绝。
	listenAddr := C.GoString(listen)
	if listenAddr == "" {
		// 使用通用环境变量名，避免依赖产品名
		listenAddr = os.Getenv("CORE_GRPC_ADDR")
	}
	if listenAddr == "" {
		listenAddr = "127.0.0.1:27820"
	}

	params := hcore.SetupRequest{
		BasePath:          C.GoString(baseDir),
		WorkingDir:        C.GoString(workingDir),
		TempDir:           C.GoString(tempDir),
		FlutterStatusPort: int64(statusPort),
		Debug:             bool(debug),
		Mode:              hcore.SetupMode(mode),
		Listen:            listenAddr,
		Secret:            C.GoString(secret),
	}
	err := hcore.Setup(&params, nil)
	return emptyOrErrorC(err)
}

//export freeString
func freeString(str *C.char) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	C.free(unsafe.Pointer(str))
}

//export start
func start(configPath *C.char, disableMemoryLimit bool) *C.char {
	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

	_, err := hcore.Start(&hcore.StartRequest{
		ConfigPath:             C.GoString(configPath),
		EnableOldCommandServer: true,
		DisableMemoryLimit:     bool(disableMemoryLimit),
	})
	return emptyOrErrorC(err)
}

//export stop
func stop() *C.char {
	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

	_, err := hcore.Stop()
	return emptyOrErrorC(err)
}

//export restart
func restart(configPath *C.char, disableMemoryLimit bool) *C.char {
	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

	_, err := hcore.Restart(&hcore.StartRequest{
		ConfigPath:             C.GoString(configPath),
		EnableOldCommandServer: true,
		DisableMemoryLimit:     bool(disableMemoryLimit),
	})
	return emptyOrErrorC(err)
}

//export GetServerPublicKey
func GetServerPublicKey() *C.char {
	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

	publicKey := hcore.GetGrpcServerPublicKey()
	return C.CString(string(publicKey)) // Return as C string, caller must free
}

//export AddGrpcClientPublicKey
func AddGrpcClientPublicKey(clientPublicKey *C.char) *C.char {
	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

	// Convert C string to Go byte slice
	clientKey := C.GoBytes(unsafe.Pointer(clientPublicKey), C.int(len(C.GoString(clientPublicKey))))
	err := hcore.AddGrpcClientPublicKey(clientKey)
	return emptyOrErrorC(err)
}

//export closeGrpc
func closeGrpc(mode C.int) {
	// runtime.LockOSThread()
	// defer runtime.UnlockOSThread()

	hcore.Close(hcore.SetupMode(mode))
}

//export parseConfig
func parseConfig(configPath *C.char, tempPath *C.char, debug bool) *C.char {
	resp, err := hcore.Parse(&hcore.ParseRequest{
		ConfigPath: C.GoString(configPath),
		TempPath:   C.GoString(tempPath),
		Debug:      bool(debug),
	})
	if err != nil {
		message := err.Error()
		if resp != nil && resp.Message != "" {
			message = resp.Message
		}
		return jsonResult(false, message, resp)
	}
	return jsonResult(true, "", resp)
}

//export generateConfig
func generateConfig(configPath *C.char) *C.char {
	resp, err := hcore.GenerateConfig(&hcore.GenerateConfigRequest{
		Path: C.GoString(configPath),
	})
	if err != nil {
		return jsonResult(false, err.Error(), nil)
	}
	return jsonResult(true, "", resp)
}

//export changeCoreSettings
func changeCoreSettings(coreSettingsJSON *C.char) *C.char {
	_, err := hcore.ChangeCoreSettings(&hcore.ChangeCoreSettingsRequest{
		CoreSettingsJson: C.GoString(coreSettingsJSON),
	})
	return emptyOrErrorC(err)
}

//export getCoreInfo
func getCoreInfo() *C.char {
	return jsonResult(true, "", hcore.SnapshotCoreInfo())
}

//export getSystemInfo
func getSystemInfo() *C.char {
	info := hcore.SnapshotSystemInfo()
	if info == nil {
		return jsonResult(false, "core service is not started", nil)
	}
	return jsonResult(true, "", info)
}

//export getOutboundsInfo
func getOutboundsInfo(onlyMain bool) *C.char {
	info := hcore.SnapshotOutbounds(bool(onlyMain))
	if info == nil {
		return jsonResult(false, "core service is not started", nil)
	}
	return jsonResult(true, "", info)
}

//export selectOutbound
func selectOutbound(groupTag *C.char, outboundTag *C.char) *C.char {
	resp, err := hcore.SelectOutbound(&hcore.SelectOutboundRequest{
		GroupTag:    C.GoString(groupTag),
		OutboundTag: C.GoString(outboundTag),
	})
	if err != nil {
		message := err.Error()
		if resp != nil && resp.Message != "" {
			message = resp.Message
		}
		return jsonResult(false, message, resp)
	}
	return jsonResult(true, "", resp)
}

//export urlTest
func urlTest(groupTag *C.char) *C.char {
	resp, err := hcore.UrlTest(&hcore.UrlTestRequest{
		GroupTag: C.GoString(groupTag),
	})
	if err != nil {
		message := err.Error()
		if resp != nil && resp.Message != "" {
			message = resp.Message
		}
		return jsonResult(false, message, resp)
	}
	return jsonResult(true, "", resp)
}

//export setSystemProxyEnabled
func setSystemProxyEnabled(enabled bool) *C.char {
	resp, err := hcore.SetSystemProxyEnabled(context.Background(), &hcore.SetSystemProxyEnabledRequest{
		IsEnabled: bool(enabled),
	})
	if err != nil {
		message := err.Error()
		if resp != nil && resp.Message != "" {
			message = resp.Message
		}
		return jsonResult(false, message, resp)
	}
	return jsonResult(true, "", resp)
}

//export generateWarpConfig
func generateWarpConfig(licenseKey *C.char, accountID *C.char, accessToken *C.char) *C.char {
	resp, err := hcore.GenerateWarpConfig(&hcore.GenerateWarpConfigRequest{
		LicenseKey:  C.GoString(licenseKey),
		AccountId:   C.GoString(accountID),
		AccessToken: C.GoString(accessToken),
	})
	if err != nil {
		return jsonResult(false, err.Error(), nil)
	}
	return jsonResult(true, "", resp)
}

package hcore

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/reddts/edgegate-core/v2/config"
	hcommon "github.com/reddts/edgegate-core/v2/hcommon"
	adapter "github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing-box/protocol/group"
	common "github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
	"github.com/sagernet/sing/service"
)

func readStatus(prev *SystemInfo) *SystemInfo {
	var message SystemInfo
	message.Memory = int64(memory.Inuse())
	message.Goroutines = int32(runtime.NumGoroutine())

	if static.Box != nil {
		if clashServer := service.FromContext[adapter.ClashServer](static.Box.Context()); clashServer != nil {
			message.TrafficAvailable = true
			trafficManager := clashServer.(*clashapi.Server).TrafficManager()
			message.UplinkTotal, message.DownlinkTotal = trafficManager.Total()
			message.ConnectionsIn = int32(trafficManager.ConnectionsLen())
			if prev != nil {
				message.Uplink = message.UplinkTotal - prev.UplinkTotal
				message.Downlink = message.DownlinkTotal - prev.DownlinkTotal
			}
		}

		if currentOutBound, ok := static.Box.GetInstance().Outbound().Outbound(config.OutboundSelectTag); ok {
			if selectOutBound, ok := currentOutBound.(*group.Selector); ok {
				message.CurrentOutbound = TrimTagName(selectOutBound.Now())
			}
		}
		if message.CurrentOutbound == config.OutboundURLTestTag {
			if currentOutBound, ok := static.Box.GetInstance().Outbound().Outbound(config.OutboundURLTestTag); ok {
				if urlTestGroup, ok := currentOutBound.(*group.URLTest); ok {
					message.CurrentOutbound = fmt.Sprint(message.CurrentOutbound, "->", TrimTagName(urlTestGroup.Now()))
				}
			}
		}

		if prev != nil && prev.CurrentProfile != "" && message.UplinkTotal >= 1000000 {
			message.CurrentProfile = prev.CurrentProfile
		} else if lastName := static.LastStartRequestName(); lastName != "" {
			message.CurrentProfile = lastName
		} else if prev != nil {
			message.CurrentProfile = prev.CurrentProfile
		}
	}

	return &message
}

// func (s *CoreRPCServer) OutboundsInfo(req *hcommon.Empty, stream grpc.ServerStreamingServer[OutboundGroupList]) error {
// 	if groupClient == nil {
// 		groupClient = libbox.NewCommandClient(
// 			&CommandClientHandler{
// 				command: libbox.CommandGroup,
// 				// port:   s.port,
// 			},
// 			&libbox.CommandClientOptions{
// 				Command:        libbox.CommandGroup,
// 				StatusInterval: 500000000, // 500ms debounce
// 			},
// 		)

// 		defer func() {
// 			groupClient.Disconnect()
// 			groupClient = nil
// 		}()

// 		groupClient.Connect()
// 	}

// 	sub, done, _ := outboundsInfoObserver.Subscribe()

// 	for {
// 		select {
// 		case <-stream.Context().Done():
// 			return nil
// 		case <-done:
// 			return nil
// 		case info := <-sub:
// 			stream.Send(info)
// 			// case <-time.After(500 * time.Millisecond):
// 		}
// 	}
// }

// func (s *CoreRPCServer) MainOutboundsInfo(req *hcommon.Empty, stream grpc.ServerStreamingServer[OutboundGroupList]) error {
// 	if groupInfoOnlyClient == nil {
// 		groupInfoOnlyClient = libbox.NewCommandClient(
// 			&CommandClientHandler{
// 				command: libbox.CommandGroupInfoOnly,
// 				// port:   s.port,
// 			},
// 			&libbox.CommandClientOptions{
// 				Command:        libbox.CommandGroupInfoOnly,
// 				StatusInterval: 500000000, // 500ms debounce
// 			},
// 		)

// 		defer func() {
// 			groupInfoOnlyClient.Disconnect()
// 			groupInfoOnlyClient = nil
// 		}()
// 		groupInfoOnlyClient.Connect()
// 	}

// 	sub, stopch, _ := mainOutboundsInfoObserver.Subscribe()

// 	for {
// 		select {
// 		case <-stream.Context().Done():
// 			return nil
// 		case <-stopch:
// 			return nil
// 		case info := <-sub:
// 			stream.Send(info)
// 			// case <-time.After(500 * time.Millisecond):
// 		}
// 	}
// }

func (s *CoreRPCServer) SelectOutbound(ctx context.Context, in *SelectOutboundRequest) (*hcommon.Response, error) {
	return SelectOutbound(in)
}

func SelectOutbound(in *SelectOutboundRequest) (*hcommon.Response, error) {
	// err := libbox.NewStandaloneCommandClient().SelectOutbound(in.GroupTag, in.OutboundTag)
	// if err != nil {
	// 	return &hcommon.Response{
	// 		Code:    hcommon.ResponseCode_FAILED,
	// 		Message: err.Error(),
	// 	}, err
	// }

	// return &hcommon.Response{
	// 	Code:    hcommon.ResponseCode_OK,
	// 	Message: "",
	// }, nil
	Log(LogLevel_DEBUG, LogType_CORE, "select outbound: ", in.GroupTag, " -> ", in.OutboundTag)
	outboundGroup, isLoaded := static.Box.GetInstance().Outbound().Outbound(in.GroupTag)
	if !isLoaded {
		return &hcommon.Response{
			Code:    hcommon.ResponseCode_FAILED,
			Message: E.New("selector not found: ", in.GroupTag).Error(),
		}, E.New("selector not found: ", in.GroupTag)
	}
	selector, isSelector := outboundGroup.(*group.Selector)
	if !isSelector {
		return &hcommon.Response{
			Code:    hcommon.ResponseCode_FAILED,
			Message: E.New("outbound is not a selector: ", in.GroupTag).Error(),
		}, E.New("outbound is not a selector: ", in.GroupTag)
	}
	if !selector.SelectOutbound(in.OutboundTag) {
		return &hcommon.Response{
			Code:    hcommon.ResponseCode_FAILED,
			Message: E.New("outbound not found in selector:: ", in.GroupTag).Error(),
		}, E.New("outbound not found in selector: ", in.GroupTag)
	}
	Log(LogLevel_DEBUG, LogType_CORE, "Trying to ping outbound: ", in.OutboundTag)
	markFFIOutboundsDirty()
	go func() {
		for _, detour := range static.Box.GetInstance().Outbound().Outbounds() {
			if urlTest, ok := detour.(*group.URLTest); ok {
				urlTest.CheckOutbounds()
				break
			}
		}
		refreshAllFFIOutbounds()
		refreshFFIStatus()
	}()
	refreshAllFFIOutbounds()
	refreshFFIStatus()
	return &hcommon.Response{
		Code:    hcommon.ResponseCode_OK,
		Message: "",
	}, nil
}

func (s *CoreRPCServer) UrlTest(ctx context.Context, in *UrlTestRequest) (*hcommon.Response, error) {
	return UrlTest(in)
}

func UrlTest(in *UrlTestRequest) (*hcommon.Response, error) {
	// err := libbox.NewStandaloneCommandClient().URLTest(in.GroupTag)
	// if err != nil {
	// 	return &hcommon.Response{
	// 		Code:    hcommon.ResponseCode_FAILED,
	// 		Message: err.Error(),
	// 	}, err
	// }

	// return &hcommon.Response{
	// 	Code:    hcommon.ResponseCode_OK,
	// 	Message: "",
	// }, nil

	groupTag := in.GroupTag

	if static.Box == nil {
		return nil, E.New("service not ready")
	}
	outboundManager := static.Box.GetInstance().Outbound()
	abstractOutboundGroup, isLoaded := outboundManager.Outbound(groupTag)
	if !isLoaded {
		return &hcommon.Response{
			Code:    hcommon.ResponseCode_FAILED,
			Message: E.New("outbound group not found: ", in.GroupTag).Error(),
		}, E.New("outbound group not found: ", groupTag)
	}
	outboundGroup, isOutboundGroup := abstractOutboundGroup.(adapter.OutboundGroup)
	if !isOutboundGroup {
		return &hcommon.Response{
			Code:    hcommon.ResponseCode_FAILED,
			Message: E.New("outbound is not a group: ", in.GroupTag).Error(),
		}, E.New("outbound is not a group: ", groupTag)
	}

	if urlTest, isURLTest := abstractOutboundGroup.(*group.URLTest); isURLTest {
		markFFIOutboundsDirty()
		go func() {
			urlTest.CheckOutbounds()
			refreshAllFFIOutbounds()
			refreshFFIStatus()
		}()
	} else {
		historyStorage := static.Box.UrlTestHistory()
		outbounds := common.Filter(common.Map(outboundGroup.All(), func(it string) adapter.Outbound {
			itOutbound, _ := outboundManager.Outbound(it)
			return itOutbound
		}), func(it adapter.Outbound) bool {
			if it == nil {
				return false
			}
			_, isGroup := it.(adapter.OutboundGroup)
			return !isGroup
		})
		b, _ := batch.New(static.Box.Context(), batch.WithConcurrencyNum[any](10))
		for _, detour := range outbounds {
			outboundToTest := detour
			outboundTag := outboundToTest.Tag()
			b.Go(outboundTag, func() (any, error) {
				t, err := urltest.URLTest(static.Box.Context(), "", outboundToTest)
				if err != nil {
					historyStorage.DeleteURLTestHistory(outboundTag)
				} else {
					historyStorage.StoreURLTestHistory(outboundTag, &adapter.URLTestHistory{
						Time:  time.Now(),
						Delay: t,
					})
				}
				return nil, nil
			})
		}
		markFFIOutboundsDirty()
		go func() {
			_ = b.Wait()
			refreshAllFFIOutbounds()
			refreshFFIStatus()
		}()
	}

	return &hcommon.Response{
		Code:    hcommon.ResponseCode_OK,
		Message: "",
	}, nil
}

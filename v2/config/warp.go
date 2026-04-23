package config

import (
	"encoding/base64"
	"fmt"
	"net/netip"

	"github.com/bepass-org/warp-plus/warp"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func wireGuardToSingbox(wgConfig WarpWireguardConfig, server string, port uint16) (*option.Outbound, error) {
	clientID, _ := base64.StdEncoding.DecodeString(wgConfig.ClientID)
	if len(clientID) < 3 {
		clientID = []byte{0, 0, 0}
	}
	wg := &option.WireGuardEndpointOptions{
		Name:       "WARP",
		PrivateKey: wgConfig.PrivateKey,
		MTU:        1330,
		Peers: []option.WireGuardPeer{
			{
				Address:   server,
				Port:      port,
				PublicKey: wgConfig.PeerPublicKey,
				Reserved:  []uint8{clientID[0], clientID[1], clientID[2]},
			},
		},
	}
	for _, addr := range []string{wgConfig.LocalAddressIPv4 + "/24", wgConfig.LocalAddressIPv6 + "/128"} {
		if addr == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(addr)
		if err != nil {
			return nil, err
		}
		wg.Address = append(wg.Address, prefix)
	}
	out := &option.Outbound{Type: C.TypeWireGuard, Tag: "WARP", Options: wg}
	return out, nil
}

func getRandomIP() string {
	return "162.159.192.1"
}

func generateWarp(license string, host string, port uint16, fakePackets string, fakePacketsSize string, fakePacketsDelay string, fakePacketsMode string) (*option.Outbound, error) {
	_, _, wgConfig, err := GenerateWarpInfo(license, "", "")
	if err != nil {
		return nil, err
	}
	if wgConfig == nil {
		return nil, fmt.Errorf("invalid warp config")
	}
	return GenerateWarpSingbox(*wgConfig, host, port, fakePackets, fakePacketsSize, fakePacketsDelay, fakePacketsMode)
}

func GenerateWarpSingbox(wgConfig WarpWireguardConfig, host string, port uint16, fakePackets string, fakePacketsSize string, fakePacketsDelay string, fakePacketMode string) (*option.Outbound, error) {
	if host == "" {
		host = getRandomIP()
	}
	if host == "auto" || host == "auto4" || host == "auto6" {
		host = getRandomIP()
	}
	return wireGuardToSingbox(wgConfig, host, port)
}

func GenerateWarpInfo(license string, oldAccountId string, oldAccessToken string) (*warp.Identity, string, *WarpWireguardConfig, error) {
	_ = license
	_ = oldAccountId
	_ = oldAccessToken
	return nil, "", nil, fmt.Errorf("warp generation is disabled during libbox official migration")
}

func getOrGenerateWarpLocallyIfNeeded(warpOptions *WarpOptions) WarpWireguardConfig {
	if warpOptions == nil {
		return WarpWireguardConfig{}
	}
	if warpOptions.WireguardConfig.PrivateKey != "" {
		return warpOptions.WireguardConfig
	}
	return WarpWireguardConfig{}
}

func patchWarp(base *option.Outbound, configOpt *CoreOptions, final bool, staticIpsDns map[string][]string) error {
	_ = base
	_ = configOpt
	_ = final
	_ = staticIpsDns
	return nil
}

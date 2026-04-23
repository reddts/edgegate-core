package config

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

type outboundMap map[string]interface{}

func patchOutboundMux(base option.Outbound, configOpt CoreOptions, obj outboundMap) outboundMap {
	return obj
}

func patchOutboundTLSTricks(base option.Outbound, configOpt CoreOptions, obj outboundMap) outboundMap {
	return obj
}

func patchOutboundFragment(base option.Outbound, configOpt CoreOptions, obj outboundMap) outboundMap {
	return obj
}

func isOutboundReality(base option.Outbound) bool {
	return false
}

func patchOutbound(base option.Outbound, configOpt CoreOptions, dns *option.DNSOptions) (*option.Outbound, error) {
	_ = patchWarp(&base, &configOpt, true, nil)
	if base.Type == C.TypeBlock || base.Type == C.TypeSelector || base.Type == C.TypeURLTest {
		return &base, nil
	}
	return &base, nil
}

func patchOutboundXray(base option.Outbound, configOpt CoreOptions, obj outboundMap, staticIpsDns map[string][]string) outboundMap {
	return obj
}

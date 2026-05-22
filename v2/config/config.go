package config

import (
	context "context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"net/netip"
	"net/url"
	"runtime"
	"strings"
	sync "sync"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	dns "github.com/sagernet/sing-dns"
	"github.com/sagernet/sing/common/json/badoption"
)

const (
	DNSRemoteTag       = "dns-remote"
	DNSLocalTag        = "dns-local"
	DNSDirectTag       = "dns-direct"
	DNSBlockTag        = "dns-block"
	DNSFakeTag         = "dns-fake"
	DNSTricksDirectTag = "dns-trick-direct"

	OutboundDirectTag         = "direct 搂hide搂"
	OutboundBypassTag         = "bypass 搂hide搂"
	OutboundBlockTag          = "block 搂hide搂"
	OutboundSelectTag         = "select"
	OutboundURLTestTag        = "auto"
	OutboundDirectFragmentTag = "direct-fragment 搂hide搂"
	WARPConfigTag             = "Edgegate Warp"

	InboundTUNTag   = "tun-in"
	InboundMixedTag = "mixed-in"
	InboundDNSTag   = "dns-in"
)

var (
	OutboundMainProxyTag   = OutboundSelectTag
	PredefinedOutboundTags = []string{OutboundDirectTag, OutboundBypassTag, OutboundBlockTag, OutboundSelectTag, OutboundURLTestTag, OutboundDirectFragmentTag}
)

func BuildConfigJson(configOpt CoreOptions, input option.Options) (string, error) {
	options, err := BuildConfig(configOpt, input)
	if err != nil {
		return "", err
	}
	return ToJson(*options)
}

// GenerateRuntimeOptions builds a runnable sing-box config from legacy-converted
// outbounds and core options. Runtime entrypoints should prefer this API instead
// of the old BuildConfig/BuildConfigJson compatibility wrappers.
func GenerateRuntimeOptions(opt CoreOptions, input option.Options) (*option.Options, error) {
	var options option.Options
	if opt.EnableFullConfig {
		options.Inbounds = input.Inbounds
		options.DNS = input.DNS
		options.Route = input.Route
	}
	options.DNS = &option.DNSOptions{}
	if opt.Warp.EnableWarp && opt.Warp.Mode == "warp_over_proxy" {
		OutboundMainProxyTag = WARPConfigTag
	} else {
		OutboundMainProxyTag = OutboundSelectTag
	}
	setClashAPI(&options, &opt)
	setLog(&options, &opt)
	setInbound(&options, &opt)
	err := setDns(&options, &opt)
	if err != nil {
		return nil, err
	}
	setNTP(&options, &opt)
	setRoutingOptions(&options, &opt)
	err = setOutbounds(&options, &input, &opt)

	if err != nil {
		return nil, err
	}
	err = setFakeDns(&options, &opt)
	if err != nil {
		return nil, err
	}
	err = addForceDirect(&options, &opt)
	if err != nil {
		return nil, err
	}
	return &options, nil
}

// TODO include selectors
func BuildConfig(opt CoreOptions, input option.Options) (*option.Options, error) {
	return GenerateRuntimeOptions(opt, input)
}

func setNTP(options *option.Options, opt *CoreOptions) {
	if options == nil {
		return
	}
	// NTP over UDP/123 is frequently blocked in desktop/mobile networks and may
	// produce noisy startup errors. Enable it only when explicitly requested.
	if opt == nil || !opt.EnableNTP {
		options.NTP = nil
		return
	}
	options.NTP = &option.NTPOptions{
		Enabled:       true,
		ServerOptions: option.ServerOptions{ServerPort: 123, Server: "time.apple.com"},
		Interval:      badoption.Duration(12 * time.Hour),
	}
}

func getHostnameIfNotIP(inp string) (string, error) {
	if inp == "" {
		return "", fmt.Errorf("empty hostname: %s", inp)
	}
	if net.ParseIP(strings.Trim(inp, "[]")) == nil {
		return inp, nil
	}
	return "", fmt.Errorf("not a hostname: %s", inp)
}

func setOutbounds(options *option.Options, input *option.Options, opt *CoreOptions) error {
	var outbounds []option.Outbound
	var tags []string
	// OutboundMainProxyTag = OutboundSelectTag
	// inbound==warp over proxies
	// outbound==proxies over warp
	if opt.Warp.EnableWarp {
		for _, out := range input.Outbounds {
			if wg := takeWireGuardOptions(out); wg != nil &&
				(wg.PrivateKey == opt.Warp.WireguardConfig.PrivateKey || wg.PrivateKey == "p1") {
				opt.Warp.EnableWarp = false
				break
			}
		}
	}
	if opt.Warp.EnableWarp && (opt.Warp.Mode == "warp_over_proxy" || opt.Warp.Mode == "proxy_over_warp") {
		wg := getOrGenerateWarpLocallyIfNeeded(&opt.Warp)
		out, err := GenerateWarpSingbox(wg, opt.Warp.CleanIP, opt.Warp.CleanPort, opt.Warp.FakePackets, opt.Warp.FakePacketSize, opt.Warp.FakePacketDelay, opt.Warp.FakePacketMode)
		if err != nil {
			return fmt.Errorf("failed to generate warp config: %v", err)
		}
		out.Tag = WARPConfigTag
		if wgOut := takeWireGuardOptions(*out); wgOut != nil {
			if opt.Warp.Mode == "warp_over_proxy" {
				wgOut.Detour = OutboundSelectTag
			} else {
				wgOut.Detour = OutboundDirectTag
			}
		}
		patchWarp(out, opt, true, nil)
		outbounds = append(outbounds, *out)
	}
	for _, out := range input.Outbounds {
		if contains(PredefinedOutboundTags, out.Tag) {
			continue
		}
		outbound, err := patchOutbound(out, *opt, options.DNS)
		if err != nil {
			return err
		}
		out = *outbound

		switch out.Type {
		case C.TypeBlock:
			continue
		case C.TypeSelector, C.TypeURLTest:
			continue
		default:
			if opt.Warp.EnableWarp && opt.Warp.Mode == "warp_over_proxy" && out.Tag == WARPConfigTag {
				continue
			}
			if contains([]string{"direct", "bypass", "block"}, out.Tag) {
				continue
			}
			if !strings.Contains(out.Tag, "搂hide搂") {
				tags = append(tags, out.Tag)
			}
			out = patchEdgegateWarpFromConfig(out, *opt)
			outbounds = append(outbounds, out)
		}
	}
	urlTest := option.Outbound{
		Type: C.TypeURLTest,
		Tag:  OutboundURLTestTag,
		Options: &option.URLTestOutboundOptions{
			Outbounds: tags,
			URL:       opt.ConnectionTestUrl,
			Interval:  badoption.Duration(opt.URLTestInterval.Duration()),
			// IdleTimeout: option.Duration(opt.URLTestIdleTimeout.Duration()),
			Tolerance:                 1,
			IdleTimeout:               badoption.Duration(opt.URLTestInterval.Duration() * 3),
			InterruptExistConnections: true,
		},
	}
	defaultSelect := urlTest.Tag

	for _, tag := range tags {
		if strings.Contains(tag, "搂default搂") {
			defaultSelect = "搂default搂"
		}
	}
	selector := option.Outbound{
		Type: C.TypeSelector,
		Tag:  OutboundSelectTag,
		Options: &option.SelectorOutboundOptions{
			Outbounds:                 append([]string{urlTest.Tag}, tags...),
			Default:                   defaultSelect,
			InterruptExistConnections: true,
		},
	}

	outbounds = append([]option.Outbound{selector, urlTest}, outbounds...)

	options.Outbounds = append(
		outbounds,
		[]option.Outbound{
			{
				Tag:  OutboundDirectTag,
				Type: C.TypeDirect,
			},
			{
				Tag:  OutboundDirectFragmentTag,
				Type: C.TypeDirect,
				Options: &option.DirectOutboundOptions{
					DialerOptions: option.DialerOptions{
						TCPFastOpen: false,
					},
				},
			},
			{
				Tag:  OutboundBypassTag,
				Type: C.TypeDirect,
			},
			{
				Tag:  OutboundBlockTag,
				Type: C.TypeBlock,
			},
		}...,
	)

	return nil
}

func isBlockedConnectionTestUrl(d string) bool {
	u, err := url.Parse(d)
	if err != nil {
		return false
	}
	return isBlockedDomain(u.Host)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func setClashAPI(options *option.Options, opt *CoreOptions) {
	if opt.EnableClashApi {
		if opt.ClashApiSecret == "" {
			opt.ClashApiSecret = generateRandomString(16)
		}
		options.Experimental = &option.ExperimentalOptions{
			ClashAPI: &option.ClashAPIOptions{
				ExternalController: fmt.Sprintf("%s:%d", "127.0.0.1", opt.ClashApiPort),
				Secret:             opt.ClashApiSecret,
			},

			CacheFile: &option.CacheFileOptions{
				Enabled: true,
				Path:    "data/clash.db",
			},
		}
	}
}

func setLog(options *option.Options, opt *CoreOptions) {
	options.Log = &option.LogOptions{
		Level:        opt.LogLevel,
		Output:       opt.LogFile,
		Disabled:     false,
		Timestamp:    false,
		DisableColor: true,
	}
}

func setInbound(options *option.Options, opt *CoreOptions) {
	var inboundDomainStrategy option.DomainStrategy
	if !opt.ResolveDestination {
		inboundDomainStrategy = option.DomainStrategy(dns.DomainStrategyAsIS)
	} else {
		inboundDomainStrategy = opt.IPv6Mode
	}
	if opt.EnableTun || opt.EnableTunService {
		tunOptions := &option.TunInboundOptions{
			Stack:                  opt.TUNStack,
			MTU:                    opt.MTU,
			AutoRoute:              true,
			StrictRoute:            opt.StrictRoute,
			EndpointIndependentNat: true,
			InboundOptions: option.InboundOptions{
				SniffEnabled:             true,
				SniffOverrideDestination: false,
				DomainStrategy:           inboundDomainStrategy,
			},
		}
		tunInbound := option.Inbound{
			Type:    C.TypeTun,
			Tag:     InboundTUNTag,
			Options: tunOptions,
		}
		switch opt.IPv6Mode {
		case option.DomainStrategy(dns.DomainStrategyUseIPv4):
			tunOptions.Address = []netip.Prefix{
				netip.MustParsePrefix("172.19.0.1/28"),
			}
		case option.DomainStrategy(dns.DomainStrategyUseIPv6):
			tunOptions.Address = []netip.Prefix{
				netip.MustParsePrefix("fdfe:dcba:9876::1/126"),
			}
		default:
			tunOptions.Address = []netip.Prefix{
				netip.MustParsePrefix("172.19.0.1/28"),
				netip.MustParsePrefix("fdfe:dcba:9876::1/126"),
			}

		}
		options.Inbounds = append(options.Inbounds, tunInbound)

	}

	var bind string
	if opt.AllowConnectionFromLAN {
		bind = "0.0.0.0"
	} else {
		bind = "127.0.0.1"
	}
	listenAddr := badoption.Addr(netip.MustParseAddr(bind))

	options.Inbounds = append(
		options.Inbounds,
		option.Inbound{
			Type: C.TypeMixed,
			Tag:  InboundMixedTag,
			Options: &option.HTTPMixedInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     &listenAddr,
					ListenPort: opt.MixedPort,
				},
				SetSystemProxy: opt.SetSystemProxy,
			},
		},
	)

	options.Inbounds = append(
		options.Inbounds,
		option.Inbound{
			Type: C.TypeDirect,
			Tag:  InboundDNSTag,
			Options: &option.DirectInboundOptions{
				ListenOptions: option.ListenOptions{
					Listen:     &listenAddr,
					ListenPort: opt.LocalDnsPort,
				},
				// OverrideAddress: "1.1.1.1",
				// OverridePort:    53,
			},
		},
	)
}

func setDns(options *option.Options, opt *CoreOptions) error {
	remoteServer, err := buildDNSServer(DNSRemoteTag, opt.RemoteDnsAddress, DNSDirectTag, opt.RemoteDnsDomainStrategy, OutboundMainProxyTag)
	if err != nil {
		return fmt.Errorf("build %s: %w", DNSRemoteTag, err)
	}
	trickDirectServer, err := buildDNSServer(DNSTricksDirectTag, "https://dns.cloudflare.com/dns-query", DNSDirectTag, opt.DirectDnsDomainStrategy, OutboundDirectFragmentTag)
	if err != nil {
		return fmt.Errorf("build %s: %w", DNSTricksDirectTag, err)
	}
	directServer, err := buildDNSServer(DNSDirectTag, opt.DirectDnsAddress, DNSLocalTag, opt.DirectDnsDomainStrategy, OutboundDirectFragmentTag)
	if err != nil {
		return fmt.Errorf("build %s: %w", DNSDirectTag, err)
	}
	localServer, err := buildDNSServer(DNSLocalTag, "local", "", option.DomainStrategy(dns.DomainStrategyAsIS), OutboundDirectTag)
	if err != nil {
		return fmt.Errorf("build %s: %w", DNSLocalTag, err)
	}

	options.DNS = &option.DNSOptions{
		RawDNSOptions: option.RawDNSOptions{
			DNSClientOptions: option.DNSClientOptions{
				IndependentCache: opt.IndependentDNSCache,
			},
			Final: DNSRemoteTag,
			Servers: []option.DNSServerOptions{
				remoteServer,
				trickDirectServer,
				directServer,
				localServer,
			},
		},
	}
	return nil
}

func buildLegacyDNSServer(tag, address, resolver string, strategy option.DomainStrategy, detour string) option.DNSServerOptions {
	return option.DNSServerOptions{
		Type: C.DNSTypeLegacy,
		Tag:  tag,
		Options: &option.LegacyDNSServerOptions{
			Address:         address,
			AddressResolver: resolver,
			Strategy:        strategy,
			Detour:          detour,
		},
	}
}

func buildDNSServer(tag, address, resolver string, strategy option.DomainStrategy, detour string) (option.DNSServerOptions, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return option.DNSServerOptions{}, fmt.Errorf("empty server address")
	}
	_ = strategy

	if address == "local" {
		return option.DNSServerOptions{
			Type: C.DNSTypeLocal,
			Tag:  tag,
			Options: &option.LocalDNSServerOptions{
				RawLocalDNSServerOptions: buildDNSDialerOptions(resolver, detour),
			},
		}, nil
	}
	if address == "fakeip" {
		return option.DNSServerOptions{
			Type:    C.DNSTypeFakeIP,
			Tag:     tag,
			Options: &option.FakeIPDNSServerOptions{},
		}, nil
	}
	if strings.HasPrefix(strings.ToLower(address), "rcode://") {
		return buildLegacyDNSServer(tag, address, resolver, strategy, detour), nil
	}
	if strings.HasPrefix(strings.ToLower(address), "dhcp://") {
		parsedURL, err := url.Parse(address)
		if err != nil {
			return option.DNSServerOptions{}, fmt.Errorf("parse DHCP address %q: %w", address, err)
		}
		dhcpOptions := &option.DHCPDNSServerOptions{}
		if parsedURL.Host != "" && parsedURL.Host != "auto" {
			dhcpOptions.Interface = parsedURL.Host
		}
		return option.DNSServerOptions{
			Type:    C.DNSTypeDHCP,
			Tag:     tag,
			Options: dhcpOptions,
		}, nil
	}

	serverType, parsedURL, err := parseDNSServerAddress(address)
	if err != nil {
		return option.DNSServerOptions{}, err
	}
	rawOptions := buildDNSDialerOptions(resolver, detour)

	switch serverType {
	case C.DNSTypeUDP, C.DNSTypeTCP:
		host, port, err := dnsServerHostPort(parsedURL, address, defaultDNSServerPort(serverType))
		if err != nil {
			return option.DNSServerOptions{}, err
		}
		return option.DNSServerOptions{
			Type: serverType,
			Tag:  tag,
			Options: &option.RemoteDNSServerOptions{
				RawLocalDNSServerOptions: rawOptions,
				DNSServerAddressOptions: option.DNSServerAddressOptions{
					Server:     host,
					ServerPort: port,
				},
			},
		}, nil
	case C.DNSTypeTLS, C.DNSTypeQUIC:
		host, port, err := dnsServerHostPort(parsedURL, address, defaultDNSServerPort(serverType))
		if err != nil {
			return option.DNSServerOptions{}, err
		}
		return option.DNSServerOptions{
			Type: serverType,
			Tag:  tag,
			Options: &option.RemoteTLSDNSServerOptions{
				RemoteDNSServerOptions: option.RemoteDNSServerOptions{
					RawLocalDNSServerOptions: rawOptions,
					DNSServerAddressOptions: option.DNSServerAddressOptions{
						Server:     host,
						ServerPort: port,
					},
				},
			},
		}, nil
	case C.DNSTypeHTTPS, C.DNSTypeHTTP3:
		host, port, err := dnsServerHostPort(parsedURL, address, defaultDNSServerPort(serverType))
		if err != nil {
			return option.DNSServerOptions{}, err
		}
		httpsOptions := &option.RemoteHTTPSDNSServerOptions{
			RemoteTLSDNSServerOptions: option.RemoteTLSDNSServerOptions{
				RemoteDNSServerOptions: option.RemoteDNSServerOptions{
					RawLocalDNSServerOptions: rawOptions,
					DNSServerAddressOptions: option.DNSServerAddressOptions{
						Server:     host,
						ServerPort: port,
					},
				},
			},
		}
		if path := normalizeDNSPath(parsedURL.Path); path != "/dns-query" {
			httpsOptions.Path = path
		}
		return option.DNSServerOptions{
			Type:    serverType,
			Tag:     tag,
			Options: httpsOptions,
		}, nil
	default:
		return option.DNSServerOptions{}, fmt.Errorf("unsupported DNS server scheme: %s", serverType)
	}
}

func buildDNSDialerOptions(resolver string, detour string) option.RawLocalDNSServerOptions {
	rawOptions := option.RawLocalDNSServerOptions{
		DialerOptions: option.DialerOptions{
			Detour: detour,
		},
	}
	if resolver != "" {
		rawOptions.DialerOptions.DomainResolver = &option.DomainResolveOptions{
			Server: resolver,
		}
	}
	return rawOptions
}

func parseDNSServerAddress(address string) (string, *url.URL, error) {
	lowerAddress := strings.ToLower(address)
	if !strings.Contains(lowerAddress, "://") {
		return C.DNSTypeUDP, nil, nil
	}
	parsedURL, err := url.Parse(address)
	if err != nil {
		return "", nil, fmt.Errorf("parse server address %q: %w", address, err)
	}
	serverType := strings.ToLower(parsedURL.Scheme)
	if serverType == "h3" {
		serverType = C.DNSTypeHTTP3
	}
	return serverType, parsedURL, nil
}

func dnsServerHostPort(parsedURL *url.URL, rawAddress string, defaultPort uint16) (string, uint16, error) {
	hostPort := rawAddress
	if parsedURL != nil {
		hostPort = parsedURL.Host
	}
	hostPort = strings.TrimSpace(hostPort)
	if hostPort == "" {
		return "", 0, fmt.Errorf("missing server host")
	}

	if parsedIP := net.ParseIP(strings.Trim(hostPort, "[]")); parsedIP != nil {
		return parsedIP.String(), defaultPort, nil
	}

	if host, port, err := net.SplitHostPort(hostPort); err == nil {
		parsedPort, parseErr := parseDNSPort(port)
		if parseErr != nil {
			return "", 0, parseErr
		}
		return host, parsedPort, nil
	}

	return hostPort, defaultPort, nil
}

func parseDNSPort(port string) (uint16, error) {
	var portNum uint16
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		return 0, fmt.Errorf("invalid server port %q", port)
	}
	if portNum == 0 {
		return 0, fmt.Errorf("invalid server port %q", port)
	}
	return portNum, nil
}

func defaultDNSServerPort(serverType string) uint16 {
	switch serverType {
	case C.DNSTypeTLS, C.DNSTypeQUIC:
		return 853
	case C.DNSTypeHTTPS, C.DNSTypeHTTP3:
		return 443
	default:
		return 53
	}
}

func normalizeDNSPath(path string) string {
	if path == "" {
		return "/dns-query"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func addForceDirect(options *option.Options, opt *CoreOptions) error {
	dnsMap := make(map[string]string)

	for _, outbound := range options.Outbounds {
		outboundOptions := outbound.Options
		if outboundOptions == nil {
			continue
		}
		if server, ok := outboundOptions.(option.ServerOptionsWrapper); ok {
			serverDomain := server.TakeServerOptions().Server
			detour := OutboundDirectTag
			if dialer, ok := outboundOptions.(option.DialerOptionsWrapper); ok {
				if server_detour := dialer.TakeDialerOptions().Detour; server_detour != "" {
					detour = server_detour
				}
			}

			if host, err := getHostnameIfNotIP(serverDomain); err == nil {
				if _, ok := dnsMap[host]; !ok || detour == OutboundDirectTag {
					dnsMap[host] = detour
				}
			}
		}
	}

	if len(dnsMap) > 0 {
		unique_dns_detours := make(map[string]bool)
		for _, detour := range dnsMap {
			unique_dns_detours[detour] = true
		}

		for detour := range unique_dns_detours {
			dns_detour := "dns-direct"
			if detour != OutboundDirectTag {
				dnsServer, err := buildDNSServer("dns-"+detour, opt.RemoteDnsAddress, DNSDirectTag, opt.RemoteDnsDomainStrategy, detour)
				if err != nil {
					return fmt.Errorf("build dns server for detour %q: %w", detour, err)
				}
				dns_detour = "dns-" + detour
				options.DNS.Servers = append(options.DNS.Servers, dnsServer)
			}

			domains := []string{}
			for domain, d := range dnsMap {
				if d == detour {
					domains = append(domains, domain)
				}
			}

			if len(domains) == 0 {
				continue
			}
			options.DNS.Rules = append([]option.DNSRule{
				makeDNSRouteRule(option.RawDefaultDNSRule{Domain: domains}, dns_detour, false, nil),
			}, options.DNS.Rules...)
		}

	}

	return nil
}

func setFakeDns(options *option.Options, opt *CoreOptions) error {
	if !opt.EnableFakeDNS {
		return nil
	}
	options.DNS.FakeIP = &option.LegacyDNSFakeIPOptions{
		Enabled: true,
	}
	fakeDNSServer, err := buildDNSServer(DNSFakeTag, "fakeip", "", option.DomainStrategy(dns.DomainStrategyUseIPv4), "")
	if err != nil {
		return fmt.Errorf("build %s: %w", DNSFakeTag, err)
	}
	options.DNS.Servers = append(options.DNS.Servers,
		fakeDNSServer,
	)
	options.DNS.Rules = append(options.DNS.Rules,
		makeDNSRouteRule(option.RawDefaultDNSRule{Inbound: []string{InboundTUNTag}}, DNSFakeTag, true, nil),
	)
	return nil
}

func setRoutingOptions(options *option.Options, opt *CoreOptions) {
	routeRules := []option.Rule{}
	dnsRules := []option.DNSRule{}
	rulesets := legacyBundledRuleSets()

	isCNLine := strings.EqualFold(opt.Region, "cn")
	finalOutbound := OutboundDirectTag

	routeRules = append(routeRules,
		makeRouteHijackDNSRule(option.RawDefaultRule{Inbound: []string{InboundDNSTag}}),
		makeRouteHijackDNSRule(option.RawDefaultRule{Port: []uint16{53}}),
		makeRouteRule(option.RawDefaultRule{IPCIDR: []string{"10.10.34.0/24"}}, OutboundMainProxyTag),
	)

	if opt.BlockAds {
		routeRules = append(routeRules,
			makeRouteRule(option.RawDefaultRule{RuleSet: []string{"geosite-category-ads-all"}}, OutboundBlockTag),
		)
	}
	if isCNLine {
		routeRules = append(routeRules,
			makeRouteRule(option.RawDefaultRule{RuleSet: []string{"geoip-cn"}}, OutboundSelectTag),
			makeRouteRule(
				option.RawDefaultRule{
					RuleSet: []string{
						"geosite-cn",
						"geosite-geolocation-cn",
						"geosite-netease",
						"geosite-bilibili",
					},
				},
				OutboundSelectTag,
			),
			makeRouteRule(option.RawDefaultRule{RuleSet: []string{"geosite-geolocation-!cn"}}, OutboundDirectTag),
		)
	} else {
		routeRules = append(routeRules,
			makeRouteRule(option.RawDefaultRule{RuleSet: []string{"geosite-geolocation-!cn"}}, OutboundSelectTag),
			makeRouteRule(option.RawDefaultRule{RuleSet: []string{"geoip-cn"}}, OutboundDirectTag),
			makeRouteRule(
				option.RawDefaultRule{
					RuleSet: []string{
						"geosite-cn",
						"geosite-geolocation-cn",
						"geosite-netease",
						"geosite-bilibili",
					},
				},
				OutboundDirectTag,
			),
		)
	}

	if opt.BypassLAN {
		routeRules = append(routeRules, makeRouteRule(option.RawDefaultRule{IPIsPrivate: true}, OutboundBypassTag))
	}

	parsedURL, err := url.Parse(opt.ConnectionTestUrl)
	var dnsCPttl uint32 = 30000
	if err == nil {
		dnsRules = append(dnsRules,
			makeDNSRouteRule(option.RawDefaultDNSRule{Domain: []string{parsedURL.Host}}, DNSRemoteTag, false, &dnsCPttl),
		)
	}
	if options.NTP != nil && options.NTP.Server != "" {
		dnsRules = append(dnsRules,
			makeDNSRouteRule(option.RawDefaultDNSRule{Domain: []string{options.NTP.Server}}, DNSDirectTag, false, &dnsCPttl),
		)
		routeRules = append(routeRules, makeRouteRule(option.RawDefaultRule{Domain: []string{options.NTP.Server}}, OutboundDirectTag))
	}

	if opt.RouteOptions.BlockQuic {
		routeRules = append(routeRules,
			makeRouteRule(option.RawDefaultRule{Port: []uint16{443}, Network: []string{"udp"}}, OutboundBlockTag),
		)
	}

	options.Route = &option.RouteOptions{
		Rules:               routeRules,
		Final:               finalOutbound,
		AutoDetectInterface: true,
		RuleSet:             rulesets,
	}
	if runtime.GOOS == "android" {
		options.Route.OverrideAndroidVPN = true
	}

	if opt.EnableDNSRouting {
		options.DNS.Rules = append(options.DNS.Rules, dnsRules...)
	}
}

func legacyBundledRuleSets() []option.RuleSet {
	return []option.RuleSet{
		legacyBundledRuleSet("geoip-cn", "geoip-cn.srs"),
		legacyBundledRuleSet("geosite-cn", "geosite-cn.srs"),
		legacyBundledRuleSet("geosite-category-ads-all", "geosite-category-ads-all.srs"),
		legacyBundledRuleSet("geosite-geolocation-cn", "geosite-geolocation-cn.srs"),
		legacyBundledRuleSet("geosite-geolocation-!cn", "geosite-geolocation-!cn.srs"),
		legacyBundledRuleSet("geosite-netease", "geosite-netease.srs"),
		legacyBundledRuleSet("geosite-bilibili", "geosite-bilibili.srs"),
	}
}

func legacyBundledRuleSet(tag string, fileName string) option.RuleSet {
	return option.RuleSet{
		Type:   C.RuleSetTypeLocal,
		Tag:    tag,
		Format: C.RuleSetFormatBinary,
		LocalOptions: option.LocalRuleSet{
			Path: pathJoinRuleSet(fileName),
		},
	}
}

func pathJoinRuleSet(fileName string) string {
	return bundledRuleSetDir + "/" + fileName
}

func makeRouteRule(raw option.RawDefaultRule, outbound string) option.Rule {
	return option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: raw,
			RuleAction: option.RuleAction{
				Action:       C.RuleActionTypeRoute,
				RouteOptions: option.RouteActionOptions{Outbound: outbound},
			},
		},
	}
}

func makeRouteHijackDNSRule(raw option.RawDefaultRule) option.Rule {
	return option.Rule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultRule{
			RawDefaultRule: raw,
			RuleAction: option.RuleAction{
				Action: C.RuleActionTypeHijackDNS,
			},
		},
	}
}

func makeDNSRouteRule(raw option.RawDefaultDNSRule, server string, disableCache bool, rewriteTTL *uint32) option.DNSRule {
	return option.DNSRule{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultDNSRule{
			RawDefaultDNSRule: raw,
			DNSRuleAction: option.DNSRuleAction{
				Action: C.RuleActionTypeRoute,
				RouteOptions: option.DNSRouteActionOptions{
					Server:       server,
					DisableCache: disableCache,
					RewriteTTL:   rewriteTTL,
				},
			},
		},
	}
}

func patchEdgegateWarpFromConfig(out option.Outbound, opt CoreOptions) option.Outbound {
	if opt.Warp.EnableWarp && opt.Warp.Mode == "proxy_over_warp" {
		if dialer, ok := out.Options.(option.DialerOptionsWrapper); ok {
			d := dialer.TakeDialerOptions()
			if d.Detour == "" {
				d.Detour = WARPConfigTag
				dialer.ReplaceDialerOptions(d)
			}
		}
	}
	return out
}

func takeWireGuardOptions(out option.Outbound) *option.WireGuardEndpointOptions {
	if out.Type != C.TypeWireGuard {
		return nil
	}
	if out.Options == nil {
		return nil
	}
	if wg, ok := out.Options.(*option.WireGuardEndpointOptions); ok {
		return wg
	}
	return nil
}

var (
	ipMaps      = map[string][]string{}
	ipMapsMutex sync.Mutex
)

func getIPs(domains ...string) []string {
	var wg sync.WaitGroup
	resChan := make(chan string, len(domains)*10) // Collect both IPv4 and IPv6
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	for _, d := range domains {
		wg.Add(1)
		go func(domain string) {
			defer wg.Done()
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", domain)
			if err != nil {
				return
			}
			for _, ip := range ips {
				ipStr := ip.String()
				if !isBlockedIP(ipStr) {
					resChan <- ipStr
				}
			}
		}(d)
	}

	go func() {
		wg.Wait()
		close(resChan)
	}()

	var res []string
	for ip := range resChan {
		res = append(res, ip)
	}
	if len(res) == 0 && ipMaps[domains[0]] != nil {
		return ipMaps[domains[0]]
	}
	ipMapsMutex.Lock()
	ipMaps[domains[0]] = res
	ipMapsMutex.Unlock()

	return res
}

func isBlockedDomain(domain string) bool {
	if strings.HasPrefix("full:", domain) {
		return false
	}
	if strings.Contains(domain, "instagram") || strings.Contains(domain, "facebook") || strings.Contains(domain, "telegram") || strings.Contains(domain, "t.me") {
		return true
	}
	ips := getIPs(domain)
	if len(ips) == 0 {
		// fmt.Println(err)
		return true
	}

	// // Print the IP addresses associated with the domain
	// fmt.Printf("IP addresses for %s:\n", domain)
	// for _, ip := range ips {
	// 	if isBlockedIP(ip) {
	// 		return true
	// 	}
	// }
	return false
}

func isBlockedIP(ip string) bool {
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "2001:4188:2:600:10") {
		return true
	}
	return false
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func generateRandomString(length int) string {
	// Determine the number of bytes needed
	bytesNeeded := (length*6 + 7) / 8

	// Generate random bytes
	randomBytes := make([]byte, bytesNeeded)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "edgegate"
	}

	// Encode random bytes to base64
	randomString := base64.URLEncoding.EncodeToString(randomBytes)

	// Trim padding characters and return the string
	return randomString[:length]
}

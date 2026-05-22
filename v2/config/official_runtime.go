package config

import (
	"context"
	"net"
	"net/netip"
	"path"
	"runtime"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	SJ "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
)

const bundledRuleSetDir = "rule-set"

var bundledRuleSetFiles = map[string]struct{}{
	"geoip-cn.srs":                 {},
	"geosite-cn.srs":               {},
	"geosite-category-ads-all.srs": {},
	"geosite-geolocation-cn.srs":   {},
	"geosite-geolocation-!cn.srs":  {},
	"geosite-netease.srs":          {},
	"geosite-bilibili.srs":         {},
}

var bundledCNRuleSetTags = map[string]struct{}{
	"geoip-cn":               {},
	"geosite-cn":             {},
	"geosite-geolocation-cn": {},
	"geosite-netease":        {},
	"geosite-bilibili":       {},
}

var bundledNonCNRuleSetTags = map[string]struct{}{
	"geosite-geolocation-!cn": {},
}

func ParseOfficialOptions(content string) (*option.Options, error) {
	var options option.Options
	ctx := include.Context(context.Background())
	if err := SJ.UnmarshalContext(ctx, []byte(content), &options); err != nil {
		return nil, err
	}
	NormalizeOfficialRuntimeOptions(&options)
	return &options, nil
}

func ParseOfficialRuntimeOptions(content string, coreOptions *CoreOptions) (*option.Options, error) {
	options, err := ParseOfficialOptions(content)
	if err != nil {
		return nil, err
	}
	PrepareOfficialRuntimeOptions(options, coreOptions)
	return options, nil
}

func PrepareOfficialRuntimeOptions(options *option.Options, coreOptions *CoreOptions) {
	if options == nil {
		return
	}
	NormalizeIOSPacketTunnelMode(runtime.GOOS, coreOptions)
	sanitizeOfficialPlatformRouteOptions(options)
	sanitizeOfficialPlatformDNSOptions(options)
	sanitizeOfficialPlatformRouteDNSOptions(options)
	sanitizeOfficialPlatformInboundOptions(options, coreOptions)
	if coreOptions != nil {
		ApplyOfficialResolveDestinationPolicy(options, coreOptions)
		ApplyOfficialRouteRegionPolicy(options, coreOptions)
	}
}

func NormalizeOfficialRuntimeOptions(options *option.Options) {
	if options == nil {
		return
	}
	resolver := newOutboundLeafResolver(options.Outbounds)
	normalizeOfficialOutboundTLS(options.Outbounds)
	normalizeOfficialDNSServers(options.DNS, resolver)
	if options.Route != nil {
		for index, ruleSet := range options.Route.RuleSet {
			if localized, ok := localizeBundledRuleSet(ruleSet); ok {
				options.Route.RuleSet[index] = localized
				continue
			}
			if ruleSet.Type != C.RuleSetTypeRemote || ruleSet.RemoteOptions.DownloadDetour == "" {
				continue
			}
			resolved := resolver.Resolve(ruleSet.RemoteOptions.DownloadDetour)
			if resolved != "" && resolved != ruleSet.RemoteOptions.DownloadDetour {
				ruleSet.RemoteOptions.DownloadDetour = resolved
				options.Route.RuleSet[index] = ruleSet
			}
		}
		if options.Route.GeoIP != nil {
			options.Route.GeoIP.DownloadDetour = resolver.Resolve(options.Route.GeoIP.DownloadDetour)
		}
		if options.Route.Geosite != nil {
			options.Route.Geosite.DownloadDetour = resolver.Resolve(options.Route.Geosite.DownloadDetour)
		}
	}
	if options.Experimental != nil && options.Experimental.ClashAPI != nil {
		options.Experimental.ClashAPI.ExternalUIDownloadDetour = resolver.Resolve(
			options.Experimental.ClashAPI.ExternalUIDownloadDetour,
		)
	}
}

func normalizeOfficialOutboundTLS(outbounds []option.Outbound) {
	for index, outbound := range outbounds {
		tlsWrapper, ok := outbound.Options.(option.OutboundTLSOptionsWrapper)
		if !ok {
			continue
		}
		tlsOptions := tlsWrapper.TakeOutboundTLSOptions()
		if tlsOptions == nil || !tlsOptions.Enabled {
			continue
		}
		clone := *tlsOptions
		if clone.ServerName == "" {
			if serverWrapper, ok := outbound.Options.(option.ServerOptionsWrapper); ok {
				server := strings.TrimSpace(serverWrapper.TakeServerOptions().Server)
				if server != "" && net.ParseIP(strings.Trim(server, "[]")) == nil {
					clone.ServerName = server
				}
			}
		}
		tlsWrapper.ReplaceOutboundTLSOptions(&clone)
		outbound.Options = tlsWrapper
		outbounds[index] = outbound
	}
}

func ApplyOfficialRouteRegionPolicy(options *option.Options, coreOptions *CoreOptions) {
	if options == nil || options.Route == nil || coreOptions == nil {
		return
	}
	outboundTags := resolveOfficialRouteOutboundTags(options.Outbounds)
	isCNLine := strings.EqualFold(coreOptions.Region, "cn")
	for index, rule := range options.Route.Rules {
		options.Route.Rules[index] = rewriteOfficialRegionRule(rule, isCNLine, outboundTags)
	}
	options.Route.Final = outboundTags.direct
}

func ApplyOfficialResolveDestinationPolicy(options *option.Options, coreOptions *CoreOptions) {
	if options == nil || coreOptions == nil || !coreOptions.ResolveDestination {
		return
	}
	inboundTags := collectOfficialInboundTags(options.Inbounds)
	if len(inboundTags) == 0 {
		return
	}
	if options.Route == nil {
		options.Route = &option.RouteOptions{}
	}
	resolveRules := buildOfficialResolveRules(
		options.Route.Rules,
		inboundTags,
		normalizeOfficialResolveStrategy(coreOptions.IPv6Mode),
		pickOfficialResolveServer(options.DNS),
	)
	if len(resolveRules) == 0 {
		return
	}
	insertIndex := officialResolveRuleInsertIndex(options.Route.Rules)
	options.Route.Rules = append(options.Route.Rules[:insertIndex], append(resolveRules, options.Route.Rules[insertIndex:]...)...)
}

func localizeBundledRuleSet(ruleSet option.RuleSet) (option.RuleSet, bool) {
	if ruleSet.Type != C.RuleSetTypeRemote {
		return ruleSet, false
	}
	fileName := path.Base(ruleSet.RemoteOptions.URL)
	if _, ok := bundledRuleSetFiles[fileName]; !ok {
		return ruleSet, false
	}
	ruleSet.Type = C.RuleSetTypeLocal
	ruleSet.Format = C.RuleSetFormatBinary
	ruleSet.LocalOptions = option.LocalRuleSet{
		Path: path.Join(bundledRuleSetDir, fileName),
	}
	ruleSet.RemoteOptions = option.RemoteRuleSet{}
	return ruleSet, true
}

func ApplyOfficialAndroidRuntimeDefaults(options *option.Options) {
	if options == nil {
		return
	}
	if options.Certificate == nil {
		options.Certificate = &option.CertificateOptions{
			Store: C.CertificateStoreMozilla,
		}
	}
}

func sanitizeOfficialPlatformRouteOptions(options *option.Options) {
	if options == nil || options.Route == nil {
		return
	}
	if runtime.GOOS != "android" {
		options.Route.OverrideAndroidVPN = false
	}
}

func sanitizeOfficialPlatformDNSOptions(options *option.Options) {
	if options == nil || options.DNS == nil {
		return
	}
	if runtime.GOOS != "windows" {
		return
	}

	localTag := ensureWindowsLocalDNSServer(options.DNS)
	if localTag == "" {
		return
	}

	serverIndex := buildDNSServerIndex(options.DNS)

	if shouldUseWindowsLocalDNSFallback(options.DNS.Final, serverIndex) {
		options.DNS.Final = localTag
	}
	for index, rule := range options.DNS.Rules {
		options.DNS.Rules[index] = rewriteWindowsDNSRuleToLocal(rule, localTag, serverIndex)
	}
}

func sanitizeOfficialPlatformRouteDNSOptions(options *option.Options) {
	if options == nil || options.Route == nil || options.DNS == nil {
		return
	}
	if runtime.GOOS != "windows" {
		return
	}

	localTag := ensureWindowsLocalDNSServer(options.DNS)
	if localTag == "" {
		return
	}
	serverIndex := buildDNSServerIndex(options.DNS)

	if options.Route.DefaultDomainResolver != nil &&
		shouldUseWindowsLocalDNSFallback(options.Route.DefaultDomainResolver.Server, serverIndex) {
		options.Route.DefaultDomainResolver.Server = localTag
	}
	for index, rule := range options.Route.Rules {
		options.Route.Rules[index] = rewriteWindowsRouteRuleResolveToLocal(rule, localTag, serverIndex)
	}
}

func sanitizeOfficialPlatformInboundOptions(options *option.Options, coreOptions *CoreOptions) {
	if options == nil || coreOptions == nil {
		return
	}
	if runtime.GOOS == "ios" {
		// iOS uses NEPacketTunnel to own traffic interception. Rewriting proxy mode
		// into a desktop mixed inbound makes sing-box try DarwinSystemProxy, which
		// crashes inside the extension and bypasses the packet-tunnel path.
		return
	}
	if !isDesktopRuntime() {
		return
	}

	if !coreOptions.EnableTun && !coreOptions.EnableTunService {
		options.Inbounds = removeOfficialTunInbounds(options.Inbounds)
		ensureOfficialDesktopMixedInbound(options, coreOptions)
		ensureOfficialDesktopDNSInbound(options, coreOptions)
	}
}

func isDesktopRuntime() bool {
	switch runtime.GOOS {
	case "darwin", "ios", "linux", "windows":
		return true
	default:
		return false
	}
}

func removeOfficialTunInbounds(inbounds []option.Inbound) []option.Inbound {
	if len(inbounds) == 0 {
		return inbounds
	}
	kept := make([]option.Inbound, 0, len(inbounds))
	for _, inbound := range inbounds {
		if inbound.Type == C.TypeTun {
			continue
		}
		kept = append(kept, inbound)
	}
	return kept
}

func ensureOfficialDesktopMixedInbound(options *option.Options, coreOptions *CoreOptions) {
	listenAddr, _ := officialDesktopInboundSettings(coreOptions)
	inbound := option.Inbound{
		Type: C.TypeMixed,
		Tag:  InboundMixedTag,
		Options: &option.HTTPMixedInboundOptions{
			ListenOptions: option.ListenOptions{
				Listen:     &listenAddr,
				ListenPort: coreOptions.MixedPort,
			},
			SetSystemProxy: coreOptions.SetSystemProxy,
		},
	}
	for index, existing := range options.Inbounds {
		if existing.Type != C.TypeMixed && existing.Tag != InboundMixedTag {
			continue
		}
		options.Inbounds[index] = inbound
		return
	}
	options.Inbounds = append([]option.Inbound{inbound}, options.Inbounds...)
}

func ensureOfficialDesktopDNSInbound(options *option.Options, coreOptions *CoreOptions) {
	listenAddr, _ := officialDesktopInboundSettings(coreOptions)
	inbound := option.Inbound{
		Type: C.TypeDirect,
		Tag:  InboundDNSTag,
		Options: &option.DirectInboundOptions{
			ListenOptions: option.ListenOptions{
				Listen:     &listenAddr,
				ListenPort: coreOptions.LocalDnsPort,
			},
		},
	}
	for index, existing := range options.Inbounds {
		if existing.Type != C.TypeDirect && existing.Tag != InboundDNSTag {
			continue
		}
		options.Inbounds[index] = inbound
		return
	}
	options.Inbounds = append(options.Inbounds, inbound)
}

func officialDesktopInboundSettings(coreOptions *CoreOptions) (badoption.Addr, option.DomainStrategy) {
	bind := "127.0.0.1"
	if coreOptions.AllowConnectionFromLAN {
		bind = "0.0.0.0"
	}
	inboundDomainStrategy := option.DomainStrategy(C.DomainStrategyAsIS)
	if coreOptions.ResolveDestination {
		inboundDomainStrategy = coreOptions.IPv6Mode
	}
	return badoption.Addr(netip.MustParseAddr(bind)), inboundDomainStrategy
}

func NormalizeIOSPacketTunnelMode(goos string, coreOptions *CoreOptions) {
	if goos != "ios" || coreOptions == nil {
		return
	}
	if coreOptions.EnableTun || coreOptions.EnableTunService {
		coreOptions.SetSystemProxy = false
		return
	}
	if coreOptions.SetSystemProxy {
		coreOptions.EnableTun = true
		coreOptions.SetSystemProxy = false
	}
}

func buildDNSServerIndex(options *option.DNSOptions) map[string]option.DNSServerOptions {
	if options == nil {
		return nil
	}
	serverIndex := make(map[string]option.DNSServerOptions, len(options.Servers))
	for _, server := range options.Servers {
		if server.Tag == "" {
			continue
		}
		serverIndex[server.Tag] = server
	}
	return serverIndex
}

func ensureWindowsLocalDNSServer(options *option.DNSOptions) string {
	if options == nil {
		return ""
	}
	for _, server := range options.Servers {
		if server.Tag == "" {
			continue
		}
		if isLocalLikeDNSServer(server) {
			return server.Tag
		}
	}
	const localTag = "dns-local"
	options.Servers = append([]option.DNSServerOptions{
		{
			Type: C.DNSTypeLocal,
			Tag:  localTag,
			Options: &option.LocalDNSServerOptions{
				RawLocalDNSServerOptions: option.RawLocalDNSServerOptions{},
			},
		},
	}, options.Servers...)
	return localTag
}

func isLocalLikeDNSServer(server option.DNSServerOptions) bool {
	switch server.Type {
	case C.DNSTypeLocal, C.DNSTypeDHCP:
		return true
	default:
		return false
	}
}

func shouldUseWindowsLocalDNSFallback(tag string, serverIndex map[string]option.DNSServerOptions) bool {
	if tag == "" {
		return false
	}
	server, ok := serverIndex[tag]
	if !ok {
		return false
	}
	return isWindowsProblematicDNSServer(server)
}

func isWindowsProblematicDNSServer(server option.DNSServerOptions) bool {
	switch server.Type {
	case C.DNSTypeUDP, C.DNSTypeTCP, C.DNSTypeTLS, C.DNSTypeQUIC, C.DNSTypeHTTPS, C.DNSTypeHTTP3:
		return true
	default:
		return false
	}
}

func rewriteWindowsDNSRuleToLocal(
	rule option.DNSRule,
	localTag string,
	serverIndex map[string]option.DNSServerOptions,
) option.DNSRule {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		rewritten := rule
		rewritten.Type = C.RuleTypeDefault
		if shouldUseWindowsLocalDNSFallback(rule.DefaultOptions.DNSRuleAction.RouteOptions.Server, serverIndex) {
			rewritten.DefaultOptions.DNSRuleAction.RouteOptions.Server = localTag
		}
		return rewritten
	case C.RuleTypeLogical:
		rewritten := rule
		for index, nested := range rewritten.LogicalOptions.Rules {
			rewritten.LogicalOptions.Rules[index] = rewriteWindowsDNSRuleToLocal(nested, localTag, serverIndex)
		}
		if shouldUseWindowsLocalDNSFallback(rule.LogicalOptions.DNSRuleAction.RouteOptions.Server, serverIndex) {
			rewritten.LogicalOptions.DNSRuleAction.RouteOptions.Server = localTag
		}
		return rewritten
	default:
		return rule
	}
}

// SanitizeRuntimeOptionsForPlatform applies platform-specific safety cleanup to
// fully built runtime options right before service startup.
func SanitizeRuntimeOptionsForPlatform(options *option.Options) {
	if options == nil {
		return
	}
	sanitizeOfficialPlatformRouteOptions(options)
	sanitizeOfficialPlatformDNSOptions(options)
	sanitizeOfficialPlatformRouteDNSOptions(options)
}

func rewriteWindowsRouteRuleResolveToLocal(
	rule option.Rule,
	localTag string,
	serverIndex map[string]option.DNSServerOptions,
) option.Rule {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		rewritten := rule
		rewritten.Type = C.RuleTypeDefault
		if shouldUseWindowsLocalDNSFallback(rule.DefaultOptions.RuleAction.ResolveOptions.Server, serverIndex) {
			rewritten.DefaultOptions.RuleAction.ResolveOptions.Server = localTag
		}
		return rewritten
	case C.RuleTypeLogical:
		rewritten := rule
		for index, nested := range rewritten.LogicalOptions.Rules {
			rewritten.LogicalOptions.Rules[index] = rewriteWindowsRouteRuleResolveToLocal(nested, localTag, serverIndex)
		}
		if shouldUseWindowsLocalDNSFallback(rule.LogicalOptions.RuleAction.ResolveOptions.Server, serverIndex) {
			rewritten.LogicalOptions.RuleAction.ResolveOptions.Server = localTag
		}
		return rewritten
	default:
		return rule
	}
}

func rewriteOfficialRegionRule(rule option.Rule, isCNLine bool, outboundTags officialRouteOutboundTags) option.Rule {
	switch rule.Type {
	case "", C.RuleTypeDefault:
		rewritten := rule
		rewritten.Type = C.RuleTypeDefault
		rewritten.DefaultOptions = rewriteOfficialRegionDefaultRule(rule.DefaultOptions, isCNLine, outboundTags)
		return rewritten
	case C.RuleTypeLogical:
		rewritten := rule
		rewritten.LogicalOptions = rewriteOfficialRegionLogicalRule(rule.LogicalOptions, isCNLine, outboundTags)
		return rewritten
	default:
		return rule
	}
}

func rewriteOfficialRegionLogicalRule(rule option.LogicalRule, isCNLine bool, outboundTags officialRouteOutboundTags) option.LogicalRule {
	for index, nested := range rule.Rules {
		rule.Rules[index] = rewriteOfficialRegionRule(nested, isCNLine, outboundTags)
	}
	if !isOfficialRouteAction(rule.RuleAction) {
		return rule
	}
	if isCNComplementRule(rule) {
		rule.RuleAction.Action = C.RuleActionTypeRoute
		rule.RouteOptions.Outbound = outboundForNonCNLine(isCNLine, outboundTags)
	}
	return rule
}

func rewriteOfficialRegionDefaultRule(rule option.DefaultRule, isCNLine bool, outboundTags officialRouteOutboundTags) option.DefaultRule {
	if !isOfficialRouteAction(rule.RuleAction) {
		return rule
	}
	switch {
	case ruleTargetsAnyRuleSet(rule.RawDefaultRule, bundledCNRuleSetTags):
		rule.RuleAction.Action = C.RuleActionTypeRoute
		rule.RouteOptions.Outbound = outboundForCNRuleSet(isCNLine, outboundTags)
	case ruleTargetsAnyRuleSet(rule.RawDefaultRule, bundledNonCNRuleSetTags):
		rule.RuleAction.Action = C.RuleActionTypeRoute
		rule.RouteOptions.Outbound = outboundForNonCNRuleSet(isCNLine, outboundTags)
	}
	return rule
}

func isOfficialRouteAction(action option.RuleAction) bool {
	return action.Action == C.RuleActionTypeRoute ||
		(action.Action == "" && action.RouteOptions.Outbound != "")
}

func outboundForCNRuleSet(isCNLine bool, outboundTags officialRouteOutboundTags) string {
	if isCNLine {
		return outboundTags.selectTag()
	}
	return outboundTags.directTag()
}

func outboundForNonCNRuleSet(isCNLine bool, outboundTags officialRouteOutboundTags) string {
	if isCNLine {
		return outboundTags.directTag()
	}
	return outboundTags.selectTag()
}

func outboundForNonCNLine(isCNLine bool, outboundTags officialRouteOutboundTags) string {
	return outboundForNonCNRuleSet(isCNLine, outboundTags)
}

func ruleTargetsAnyRuleSet(rule option.RawDefaultRule, targets map[string]struct{}) bool {
	for _, tag := range rule.RuleSet {
		if _, ok := targets[tag]; ok {
			return true
		}
	}
	return false
}

func isCNComplementRule(rule option.LogicalRule) bool {
	if rule.Mode != C.LogicalTypeAnd || len(rule.Rules) == 0 {
		return false
	}
	hasBundledGeoSite := false
	hasInvertedGeoIPCN := false
	for _, nested := range rule.Rules {
		if nested.Type != C.RuleTypeDefault {
			return false
		}
		defaultRule := nested.DefaultOptions
		if !defaultRule.Invert {
			return false
		}
		if ruleTargetsAnyRuleSet(defaultRule.RawDefaultRule, bundledCNRuleSetTags) ||
			ruleTargetsAnyRuleSet(defaultRule.RawDefaultRule, bundledNonCNRuleSetTags) {
			hasBundledGeoSite = true
		}
		for _, tag := range defaultRule.RuleSet {
			if tag == "geoip-cn" {
				hasInvertedGeoIPCN = true
			}
		}
	}
	return hasBundledGeoSite && hasInvertedGeoIPCN
}

func normalizeOfficialDNSServers(options *option.DNSOptions, resolver *outboundLeafResolver) {
	if options == nil {
		return
	}
	for index, server := range options.Servers {
		options.Servers[index] = normalizeOfficialDNSServer(server, resolver)
	}
}

func normalizeOfficialDNSServer(server option.DNSServerOptions, resolver *outboundLeafResolver) option.DNSServerOptions {
	switch typed := server.Options.(type) {
	case *option.RemoteDNSServerOptions:
		clone := *typed
		clone.DialerOptions = normalizeDialerOptionsDetour(clone.DialerOptions, resolver)
		server.Options = &clone
	case option.RemoteDNSServerOptions:
		typed.DialerOptions = normalizeDialerOptionsDetour(typed.DialerOptions, resolver)
		server.Options = typed
	case *option.RemoteTLSDNSServerOptions:
		clone := *typed
		clone.DialerOptions = normalizeDialerOptionsDetour(clone.DialerOptions, resolver)
		if shouldAllowInsecureDNS(typed.RemoteDNSServerOptions, typed.OutboundTLSOptionsContainer.TLS) {
			clone.OutboundTLSOptionsContainer.TLS = ensureInsecureTLSOptions(typed.OutboundTLSOptionsContainer.TLS)
		}
		server.Options = &clone
	case option.RemoteTLSDNSServerOptions:
		typed.DialerOptions = normalizeDialerOptionsDetour(typed.DialerOptions, resolver)
		if shouldAllowInsecureDNS(typed.RemoteDNSServerOptions, typed.OutboundTLSOptionsContainer.TLS) {
			typed.OutboundTLSOptionsContainer.TLS = ensureInsecureTLSOptions(typed.OutboundTLSOptionsContainer.TLS)
		}
		server.Options = typed
	case *option.RemoteHTTPSDNSServerOptions:
		clone := *typed
		clone.DialerOptions = normalizeDialerOptionsDetour(clone.DialerOptions, resolver)
		if shouldAllowInsecureDNS(typed.RemoteDNSServerOptions, typed.OutboundTLSOptionsContainer.TLS) {
			clone.OutboundTLSOptionsContainer.TLS = ensureInsecureTLSOptions(typed.OutboundTLSOptionsContainer.TLS)
		}
		server.Options = &clone
	case option.RemoteHTTPSDNSServerOptions:
		typed.DialerOptions = normalizeDialerOptionsDetour(typed.DialerOptions, resolver)
		if shouldAllowInsecureDNS(typed.RemoteDNSServerOptions, typed.OutboundTLSOptionsContainer.TLS) {
			typed.OutboundTLSOptionsContainer.TLS = ensureInsecureTLSOptions(typed.OutboundTLSOptionsContainer.TLS)
		}
		server.Options = typed
	}
	return server
}

func normalizeDialerOptionsDetour(dialer option.DialerOptions, resolver *outboundLeafResolver) option.DialerOptions {
	if resolver == nil || dialer.Detour == "" {
		return dialer
	}
	dialer.Detour = resolver.Resolve(dialer.Detour)
	return dialer
}

func shouldAllowInsecureDNS(options option.RemoteDNSServerOptions, tlsOptions *option.OutboundTLSOptions) bool {
	if options.ServerIsDomain() || net.ParseIP(options.Server) == nil {
		return false
	}
	if tlsOptions == nil {
		return true
	}
	return tlsOptions.ServerName == "" && !tlsOptions.Insecure
}

func ensureInsecureTLSOptions(tlsOptions *option.OutboundTLSOptions) *option.OutboundTLSOptions {
	if tlsOptions == nil {
		return &option.OutboundTLSOptions{
			Enabled:  true,
			Insecure: true,
		}
	}
	clone := *tlsOptions
	clone.Enabled = true
	clone.Insecure = true
	return &clone
}

func collectOfficialInboundTags(inbounds []option.Inbound) []string {
	tags := make([]string, 0, len(inbounds))
	seen := make(map[string]struct{}, len(inbounds))
	for _, inbound := range inbounds {
		if inbound.Tag == "" {
			continue
		}
		if _, ok := seen[inbound.Tag]; ok {
			continue
		}
		seen[inbound.Tag] = struct{}{}
		tags = append(tags, inbound.Tag)
	}
	return tags
}

func normalizeOfficialResolveStrategy(strategy option.DomainStrategy) option.DomainStrategy {
	switch strategy {
	case option.DomainStrategy(C.DomainStrategyPreferIPv4),
		option.DomainStrategy(C.DomainStrategyPreferIPv6),
		option.DomainStrategy(C.DomainStrategyIPv4Only),
		option.DomainStrategy(C.DomainStrategyIPv6Only):
		return strategy
	default:
		return option.DomainStrategy(C.DomainStrategyAsIS)
	}
}

func buildOfficialResolveRules(existing []option.Rule, inboundTags []string, strategy option.DomainStrategy, server string) []option.Rule {
	rules := make([]option.Rule, 0, len(inboundTags))
	for _, inboundTag := range inboundTags {
		if hasOfficialResolveRule(existing, inboundTag) {
			continue
		}
		rules = append(rules, option.Rule{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultRule{
				RawDefaultRule: option.RawDefaultRule{
					Inbound:  []string{inboundTag},
					Protocol: []string{"dns"},
					Invert:   true,
				},
				RuleAction: option.RuleAction{
					Action: C.RuleActionTypeResolve,
					ResolveOptions: option.RouteActionResolve{
						Server:   server,
						Strategy: strategy,
					},
				},
			},
		})
	}
	return rules
}

func hasOfficialResolveRule(rules []option.Rule, inboundTag string) bool {
	for _, rule := range rules {
		if rule.Type != C.RuleTypeDefault {
			continue
		}
		if rule.DefaultOptions.RuleAction.Action != C.RuleActionTypeResolve {
			continue
		}
		if len(rule.DefaultOptions.Inbound) == 0 {
			return true
		}
		for _, tag := range rule.DefaultOptions.Inbound {
			if tag == inboundTag {
				return true
			}
		}
	}
	return false
}

func pickOfficialResolveServer(dnsOptions *option.DNSOptions) string {
	if dnsOptions == nil {
		return ""
	}
	preferred := []string{
		"alidns",
		"dns-direct",
		"dns_local",
		"dns-local",
		"local",
		"direct",
	}
	index := make(map[string]option.DNSServerOptions, len(dnsOptions.Servers))
	for _, server := range dnsOptions.Servers {
		if server.Tag == "" {
			continue
		}
		index[server.Tag] = server
	}
	for _, tag := range preferred {
		if server, ok := index[tag]; ok && server.Type != C.DNSTypeFakeIP {
			return tag
		}
	}
	for _, server := range dnsOptions.Servers {
		if server.Tag == "" || server.Type == C.DNSTypeFakeIP {
			continue
		}
		return server.Tag
	}
	return ""
}

func officialResolveRuleInsertIndex(rules []option.Rule) int {
	index := 0
	for index < len(rules) {
		switch officialRuleActionType(rules[index]) {
		case C.RuleActionTypeSniff, C.RuleActionTypeHijackDNS, C.RuleActionTypeResolve:
			index++
		default:
			return index
		}
	}
	return index
}

func officialRuleActionType(rule option.Rule) string {
	switch rule.Type {
	case C.RuleTypeDefault:
		return rule.DefaultOptions.RuleAction.Action
	case C.RuleTypeLogical:
		return rule.LogicalOptions.RuleAction.Action
	default:
		return ""
	}
}

type outboundLeafResolver struct {
	outbounds map[string]option.Outbound
}

func newOutboundLeafResolver(outbounds []option.Outbound) *outboundLeafResolver {
	index := make(map[string]option.Outbound, len(outbounds))
	for _, outbound := range outbounds {
		if outbound.Tag == "" {
			continue
		}
		index[outbound.Tag] = outbound
	}
	return &outboundLeafResolver{outbounds: index}
}

func (r *outboundLeafResolver) Resolve(tag string) string {
	if tag == "" {
		return ""
	}
	resolved := r.resolve(tag, map[string]bool{})
	if resolved == "" {
		return tag
	}
	return resolved
}

func (r *outboundLeafResolver) resolve(tag string, visited map[string]bool) string {
	if tag == "" {
		return ""
	}
	if visited[tag] {
		return ""
	}
	visited[tag] = true
	outbound, found := r.outbounds[tag]
	if !found {
		return tag
	}
	switch outbound.Type {
	case C.TypeSelector:
		return r.resolveSelector(outbound.Options, tag, visited)
	case C.TypeURLTest:
		return r.resolveURLTest(outbound.Options, tag, visited)
	default:
		return tag
	}
}

func (r *outboundLeafResolver) resolveSelector(options any, fallback string, visited map[string]bool) string {
	var selector *option.SelectorOutboundOptions
	switch typed := options.(type) {
	case *option.SelectorOutboundOptions:
		selector = typed
	case option.SelectorOutboundOptions:
		selector = &typed
	}
	if selector == nil {
		return fallback
	}
	candidates := make([]string, 0, len(selector.Outbounds)+1)
	if selector.Default != "" {
		candidates = append(candidates, selector.Default)
	}
	candidates = append(candidates, selector.Outbounds...)
	return r.resolveCandidates(candidates, fallback, visited)
}

func (r *outboundLeafResolver) resolveURLTest(options any, fallback string, visited map[string]bool) string {
	var urlTest *option.URLTestOutboundOptions
	switch typed := options.(type) {
	case *option.URLTestOutboundOptions:
		urlTest = typed
	case option.URLTestOutboundOptions:
		urlTest = &typed
	}
	if urlTest == nil {
		return fallback
	}
	return r.resolveCandidates(urlTest.Outbounds, fallback, visited)
}

func (r *outboundLeafResolver) resolveCandidates(candidates []string, fallback string, visited map[string]bool) string {
	best := ""
	for _, candidate := range candidates {
		resolved := r.resolve(candidate, cloneVisited(visited))
		if resolved == "" {
			continue
		}
		if r.isPreferredLeaf(resolved) {
			return resolved
		}
		if best == "" {
			best = resolved
		}
	}
	if best != "" {
		return best
	}
	return fallback
}

func (r *outboundLeafResolver) isPreferredLeaf(tag string) bool {
	outbound, found := r.outbounds[tag]
	if !found {
		return false
	}
	switch outbound.Type {
	case C.TypeSelector, C.TypeURLTest, C.TypeDirect, C.TypeBlock:
		return false
	default:
		return true
	}
}

func cloneVisited(visited map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(visited))
	for key, value := range visited {
		cloned[key] = value
	}
	return cloned
}

type officialRouteOutboundTags struct {
	direct  string
	select_ string
	block   string
}

func (t officialRouteOutboundTags) directTag() string {
	if t.direct != "" {
		return t.direct
	}
	return "direct"
}

func (t officialRouteOutboundTags) selectTag() string {
	if t.select_ != "" {
		return t.select_
	}
	return OutboundSelectTag
}

func (t officialRouteOutboundTags) blockTag() string {
	if t.block != "" {
		return t.block
	}
	return "block"
}

func resolveOfficialRouteOutboundTags(outbounds []option.Outbound) officialRouteOutboundTags {
	tags := officialRouteOutboundTags{}
	for _, outbound := range outbounds {
		switch outbound.Tag {
		case "direct":
			tags.direct = outbound.Tag
		case OutboundSelectTag:
			tags.select_ = outbound.Tag
		case "block":
			tags.block = outbound.Tag
		}
	}
	for _, outbound := range outbounds {
		switch outbound.Type {
		case C.TypeDirect:
			if tags.direct == "" {
				tags.direct = outbound.Tag
			}
		case C.TypeSelector:
			if tags.select_ == "" {
				tags.select_ = outbound.Tag
			}
		case C.TypeBlock:
			if tags.block == "" {
				tags.block = outbound.Tag
			}
		}
	}
	if tags.direct == "" {
		tags.direct = "direct"
	}
	if tags.select_ == "" {
		tags.select_ = OutboundSelectTag
	}
	if tags.block == "" {
		tags.block = "block"
	}
	return tags
}

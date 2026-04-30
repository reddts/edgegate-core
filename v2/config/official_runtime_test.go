package config

import (
	"runtime"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestPrepareOfficialRuntimeOptions_StripsOverrideAndroidVPNOnNonAndroid(t *testing.T) {
	options := &option.Options{
		Route: &option.RouteOptions{
			OverrideAndroidVPN: true,
		},
	}

	PrepareOfficialRuntimeOptions(options, nil)

	want := runtime.GOOS == "android"
	if got := options.Route.OverrideAndroidVPN; got != want {
		t.Fatalf("expected override_android_vpn=%v on %s, got %v", want, runtime.GOOS, got)
	}
}

func TestSanitizeRuntimeOptionsForPlatform_StripsOverrideAndroidVPNOnNonAndroid(t *testing.T) {
	options := &option.Options{
		Route: &option.RouteOptions{
			OverrideAndroidVPN: true,
		},
	}

	SanitizeRuntimeOptionsForPlatform(options)

	want := runtime.GOOS == "android"
	if got := options.Route.OverrideAndroidVPN; got != want {
		t.Fatalf("expected override_android_vpn=%v on %s, got %v", want, runtime.GOOS, got)
	}
}

func TestPrepareOfficialRuntimeOptions_WindowsDNSFallsBackToLocal(t *testing.T) {
	options := &option.Options{
		DNS: &option.DNSOptions{
			RawDNSOptions: option.RawDNSOptions{
				Final: "google",
				Servers: []option.DNSServerOptions{
					{
						Type: C.DNSTypeTLS,
						Tag:  "google",
						Options: &option.RemoteTLSDNSServerOptions{
							RemoteDNSServerOptions: option.RemoteDNSServerOptions{
								DNSServerAddressOptions: option.DNSServerAddressOptions{
									Server:     "8.8.8.8",
									ServerPort: 853,
								},
							},
						},
					},
				},
				Rules: []option.DNSRule{
					{
						Type: C.RuleTypeDefault,
						DefaultOptions: option.DefaultDNSRule{
							RawDefaultDNSRule: option.RawDefaultDNSRule{
								Domain: []string{"cp.cloudflare.com"},
							},
							DNSRuleAction: option.DNSRuleAction{
								Action: C.RuleActionTypeRoute,
								RouteOptions: option.DNSRouteActionOptions{
									Server: "google",
								},
							},
						},
					},
				},
			},
		},
		Route: &option.RouteOptions{
			DefaultDomainResolver: &option.DomainResolveOptions{
				Server: "google",
			},
			Rules: []option.Rule{
				{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound:  []string{"in"},
							Protocol: []string{"dns"},
							Invert:   true,
						},
						RuleAction: option.RuleAction{
							Action: C.RuleActionTypeResolve,
							ResolveOptions: option.RouteActionResolve{
								Server: "google",
							},
						},
					},
				},
			},
		},
	}

	PrepareOfficialRuntimeOptions(options, nil)

	if runtime.GOOS == "windows" {
		if options.DNS.Final != "dns-local" {
			t.Fatalf("expected windows DNS final to fall back to dns-local, got %q", options.DNS.Final)
		}
		if got := options.DNS.Rules[0].DefaultOptions.DNSRuleAction.RouteOptions.Server; got != "dns-local" {
			t.Fatalf("expected windows DNS rule server to fall back to dns-local, got %q", got)
		}
		if got := options.Route.DefaultDomainResolver.Server; got != "dns-local" {
			t.Fatalf("expected windows default_domain_resolver to fall back to dns-local, got %q", got)
		}
		if got := options.Route.Rules[0].DefaultOptions.RuleAction.ResolveOptions.Server; got != "dns-local" {
			t.Fatalf("expected windows route resolve rule server to fall back to dns-local, got %q", got)
		}
		foundLocal := false
		for _, server := range options.DNS.Servers {
			if server.Tag == "dns-local" && server.Type == C.DNSTypeLocal {
				foundLocal = true
				break
			}
		}
		if !foundLocal {
			t.Fatal("expected dns-local server to be injected")
		}
		return
	}

	if options.DNS.Final != "google" {
		t.Fatalf("expected non-windows DNS final to stay google, got %q", options.DNS.Final)
	}
	if got := options.DNS.Rules[0].DefaultOptions.DNSRuleAction.RouteOptions.Server; got != "google" {
		t.Fatalf("expected non-windows DNS rule server to stay google, got %q", got)
	}
	if got := options.Route.DefaultDomainResolver.Server; got != "google" {
		t.Fatalf("expected non-windows default_domain_resolver to stay google, got %q", got)
	}
	if got := options.Route.Rules[0].DefaultOptions.RuleAction.ResolveOptions.Server; got != "google" {
		t.Fatalf("expected non-windows route resolve rule server to stay google, got %q", got)
	}
}

func TestPrepareOfficialRuntimeOptions_DesktopProxyInbounds(t *testing.T) {
	options := &option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeTun,
				Tag:  "in",
				Options: &option.TunInboundOptions{
					AutoRoute:   true,
					StrictRoute: true,
				},
			},
		},
	}
	coreOptions := &CoreOptions{
		RouteOptions: RouteOptions{
			ResolveDestination: true,
			IPv6Mode:           option.DomainStrategy(C.DomainStrategyIPv4Only),
		},
		InboundOptions: InboundOptions{
			EnableTun:        false,
			EnableTunService: false,
			SetSystemProxy:   true,
			MixedPort:        7895,
			LocalDnsPort:     16450,
		},
	}

	PrepareOfficialRuntimeOptions(options, coreOptions)

	if isDesktopRuntime() {
		if len(options.Inbounds) != 2 {
			t.Fatalf("expected desktop inbounds to be rewritten to mixed+dns, got %d", len(options.Inbounds))
		}
		if options.Inbounds[0].Type != C.TypeMixed || options.Inbounds[0].Tag != InboundMixedTag {
			t.Fatalf("expected first inbound to be mixed-in, got type=%q tag=%q", options.Inbounds[0].Type, options.Inbounds[0].Tag)
		}
		mixedOptions, ok := options.Inbounds[0].Options.(*option.HTTPMixedInboundOptions)
		if !ok {
			t.Fatalf("expected mixed inbound options type, got %T", options.Inbounds[0].Options)
		}
		if mixedOptions.ListenPort != 7895 {
			t.Fatalf("expected mixed listen port 7895, got %d", mixedOptions.ListenPort)
		}
		if !mixedOptions.SetSystemProxy {
			t.Fatal("expected mixed inbound to enable system proxy")
		}
		if got := mixedOptions.InboundOptions.DomainStrategy; got != option.DomainStrategy(C.DomainStrategyIPv4Only) {
			t.Fatalf("expected mixed inbound domain strategy ipv4_only, got %q", got)
		}
		if options.Inbounds[1].Type != C.TypeDirect || options.Inbounds[1].Tag != InboundDNSTag {
			t.Fatalf("expected second inbound to be dns direct inbound, got type=%q tag=%q", options.Inbounds[1].Type, options.Inbounds[1].Tag)
		}
		directOptions, ok := options.Inbounds[1].Options.(*option.DirectInboundOptions)
		if !ok {
			t.Fatalf("expected direct inbound options type, got %T", options.Inbounds[1].Options)
		}
		if directOptions.ListenPort != 16450 {
			t.Fatalf("expected local dns listen port 16450, got %d", directOptions.ListenPort)
		}
		return
	}

	if len(options.Inbounds) != 1 || options.Inbounds[0].Type != C.TypeTun {
		t.Fatalf("expected non-desktop inbounds to remain unchanged, got %#v", options.Inbounds)
	}
}

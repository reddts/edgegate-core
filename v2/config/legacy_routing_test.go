package config

import (
	"runtime"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestBuildConfigAppliesCNLineRoutingPolicy(t *testing.T) {
	coreOptions := *DefaultCoreOptions()
	coreOptions.Region = "cn"
	options, err := BuildConfig(
		coreOptions,
		option.Options{
			Outbounds: []option.Outbound{
				{Type: C.TypeVMess, Tag: "proxy-a"},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildConfig failed: %v", err)
	}
	if options.Route == nil {
		t.Fatal("expected route options")
	}
	if got := options.Route.Final; got != OutboundDirectTag {
		t.Fatalf("expected final outbound direct, got %q", got)
	}
	assertLegacyRouteOutbound(t, options.Route.Rules, "geoip-cn", OutboundSelectTag)
	assertLegacyRouteOutbound(t, options.Route.Rules, "geosite-cn", OutboundSelectTag)
	assertLegacyRouteOutbound(t, options.Route.Rules, "geosite-geolocation-!cn", OutboundDirectTag)
	if len(options.Route.RuleSet) == 0 {
		t.Fatal("expected bundled rule sets to be configured")
	}
}

func TestBuildConfigAppliesNonCNLineRoutingPolicy(t *testing.T) {
	coreOptions := *DefaultCoreOptions()
	coreOptions.Region = "other"
	options, err := BuildConfig(
		coreOptions,
		option.Options{
			Outbounds: []option.Outbound{
				{Type: C.TypeVMess, Tag: "proxy-a"},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildConfig failed: %v", err)
	}
	if options.Route == nil {
		t.Fatal("expected route options")
	}
	if got := options.Route.Final; got != OutboundDirectTag {
		t.Fatalf("expected final outbound direct, got %q", got)
	}
	assertLegacyRouteOutbound(t, options.Route.Rules, "geosite-geolocation-!cn", OutboundSelectTag)
	assertLegacyRouteOutbound(t, options.Route.Rules, "geoip-cn", OutboundDirectTag)
	assertLegacyRouteOutbound(t, options.Route.Rules, "geosite-cn", OutboundDirectTag)
}

func TestBuildConfigOnlyEnablesOverrideAndroidVPNOnAndroid(t *testing.T) {
	coreOptions := *DefaultCoreOptions()
	options, err := BuildConfig(
		coreOptions,
		option.Options{
			Outbounds: []option.Outbound{
				{Type: C.TypeVMess, Tag: "proxy-a"},
			},
		},
	)
	if err != nil {
		t.Fatalf("BuildConfig failed: %v", err)
	}
	if options.Route == nil {
		t.Fatal("expected route options")
	}
	want := runtime.GOOS == "android"
	if got := options.Route.OverrideAndroidVPN; got != want {
		t.Fatalf("expected override_android_vpn=%v on %s, got %v", want, runtime.GOOS, got)
	}
}

func assertLegacyRouteOutbound(t *testing.T, rules []option.Rule, ruleSetTag string, outbound string) {
	t.Helper()
	for _, rule := range rules {
		if rule.Type != C.RuleTypeDefault {
			continue
		}
		for _, tag := range rule.DefaultOptions.RuleSet {
			if tag != ruleSetTag {
				continue
			}
			if got := rule.DefaultOptions.RouteOptions.Outbound; got != outbound {
				t.Fatalf("expected ruleset %q to route to %q, got %q", ruleSetTag, outbound, got)
			}
			return
		}
	}
	t.Fatalf("expected route rule for ruleset %q", ruleSetTag)
}

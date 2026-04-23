package config

import (
	"os"
	"path/filepath"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func loadOfficialSubscriptionFixture(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("testdata", "official_subscription.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(content)
}

func TestOfficialSubscriptionFixture_ParseOfficialOptions(t *testing.T) {
	t.Parallel()

	options, err := ParseOfficialOptions(loadOfficialSubscriptionFixture(t))
	if err != nil {
		t.Fatalf("ParseOfficialOptions failed: %v", err)
	}

	if options.DNS == nil {
		t.Fatal("expected DNS options")
	}
	if options.Route == nil {
		t.Fatal("expected route options")
	}

	serverTypes := map[string]int{}
	for _, server := range options.DNS.Servers {
		serverTypes[server.Type]++
	}
	for _, wantType := range []string{
		C.DNSTypeLocal,
		C.DNSTypeQUIC,
		C.DNSTypeTLS,
		C.DNSTypeHTTPS,
		C.DNSTypeFakeIP,
	} {
		if serverTypes[wantType] == 0 {
			t.Fatalf("expected fixture to contain DNS server type %q, got %#v", wantType, serverTypes)
		}
	}

	hasPredefinedDNSRule := false
	hasRouteDNSRule := false
	hasLogicalDNSRule := false
	for _, rule := range options.DNS.Rules {
		switch rule.Type {
		case "", C.RuleTypeDefault:
			switch rule.DefaultOptions.DNSRuleAction.Action {
			case C.RuleActionTypePredefined:
				hasPredefinedDNSRule = true
			case C.RuleActionTypeRoute:
				hasRouteDNSRule = true
			}
		case C.RuleTypeLogical:
			hasLogicalDNSRule = true
			if rule.LogicalOptions.DNSRuleAction.Action == C.RuleActionTypeRoute {
				hasRouteDNSRule = true
			}
		}
	}
	if !hasPredefinedDNSRule {
		t.Fatal("expected fixture to contain dns.rules[].action=predefined")
	}
	if !hasRouteDNSRule {
		t.Fatal("expected fixture to contain dns.rules[].action=route")
	}
	if !hasLogicalDNSRule {
		t.Fatal("expected fixture to contain logical DNS rule")
	}

	hasHijackDNS := false
	hasLogicalRouteRule := false
	for _, rule := range options.Route.Rules {
		switch rule.Type {
		case "", C.RuleTypeDefault:
			if rule.DefaultOptions.RuleAction.Action == C.RuleActionTypeHijackDNS {
				hasHijackDNS = true
			}
		case C.RuleTypeLogical:
			hasLogicalRouteRule = true
		}
	}
	if !hasHijackDNS {
		t.Fatal("expected fixture to contain route.rules[].action=hijack-dns")
	}
	if !hasLogicalRouteRule {
		t.Fatal("expected fixture to contain logical route rule")
	}

	if len(options.Route.RuleSet) != 7 {
		t.Fatalf("expected 7 bundled rule sets, got %d", len(options.Route.RuleSet))
	}
	for _, ruleSet := range options.Route.RuleSet {
		if ruleSet.Type != C.RuleSetTypeLocal {
			t.Fatalf("expected bundled rule set %q to localize, got type=%q", ruleSet.Tag, ruleSet.Type)
		}
		if ruleSet.LocalOptions.Path == "" {
			t.Fatalf("expected bundled rule set %q to have local path", ruleSet.Tag)
		}
	}
}

func TestOfficialSubscriptionFixture_ParseOfficialRuntimeOptions(t *testing.T) {
	t.Parallel()

	coreOptions := DefaultCoreOptions()
	options, err := ParseOfficialRuntimeOptions(
		loadOfficialSubscriptionFixture(t),
		coreOptions,
	)
	if err != nil {
		t.Fatalf("ParseOfficialRuntimeOptions failed: %v", err)
	}

	if options.Certificate != nil {
		t.Fatalf("config-layer runtime parse should not inject mobile-only certificate defaults, got %#v", options.Certificate)
	}

	if options.Route == nil {
		t.Fatal("expected route options")
	}

	hasResolveRule := false
	for _, rule := range options.Route.Rules {
		if rule.Type != C.RuleTypeDefault {
			continue
		}
		if rule.DefaultOptions.RuleAction.Action != C.RuleActionTypeResolve {
			continue
		}
		hasResolveRule = true
		break
	}
	if !hasResolveRule {
		t.Fatal("expected runtime parse to keep or inject at least one resolve rule")
	}

	selectorFound := false
	urltestFound := false
	for _, outbound := range options.Outbounds {
		switch outbound.Type {
		case C.TypeSelector:
			selectorFound = true
		case C.TypeURLTest:
			urltestFound = true
		}
	}
	if !selectorFound || !urltestFound {
		t.Fatalf("expected selector+urltest outbounds, selector=%v urltest=%v", selectorFound, urltestFound)
	}
}

func TestOfficialSubscriptionFixture_LocalizedRuleSetsRemainRunnable(t *testing.T) {
	t.Parallel()

	options, err := ParseOfficialOptions(loadOfficialSubscriptionFixture(t))
	if err != nil {
		t.Fatalf("ParseOfficialOptions failed: %v", err)
	}

	for _, ruleSet := range options.Route.RuleSet {
		if ruleSet.Type != C.RuleSetTypeLocal {
			t.Fatalf("expected rule set %q to be local, got %q", ruleSet.Tag, ruleSet.Type)
		}
		if filepath.Dir(ruleSet.LocalOptions.Path) != bundledRuleSetDir {
			t.Fatalf("expected rule set %q path to be under %q, got %q", ruleSet.Tag, bundledRuleSetDir, ruleSet.LocalOptions.Path)
		}
	}

	for _, server := range options.DNS.Servers {
		if server.Type == C.DNSTypeLegacy {
			t.Fatalf("expected typed DNS servers only in fixture, found legacy server tag=%q", server.Tag)
		}
	}

	var _ *option.Options = options
}

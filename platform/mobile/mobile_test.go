package mobile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cfg "github.com/reddts/edgegate-core/v2/config"
	C "github.com/sagernet/sing-box/constant"
)

func writeOfficialSubscriptionFixture(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "..", "v2", "config", "testdata", "official_subscription.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "official_subscription.json")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func marshalCoreOptions(t *testing.T, options *cfg.CoreOptions) string {
	t.Helper()

	content, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("marshal core options: %v", err)
	}
	return string(content)
}

func TestValidateConfig_OfficialSubscriptionFixture(t *testing.T) {
	t.Parallel()

	path := writeOfficialSubscriptionFixture(t)
	coreOptions := cfg.DefaultCoreOptions()
	coreOptions.ExecuteConfigAsIs = true
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := ValidateConfig(path, marshalCoreOptions(t, coreOptions)); err != nil {
		t.Fatalf("ValidateConfig failed: %v", err)
	}
}

func TestBuildConfig_OfficialSubscriptionFixture(t *testing.T) {
	t.Parallel()

	path := writeOfficialSubscriptionFixture(t)
	coreOptions := cfg.DefaultCoreOptions()
	coreOptions.ExecuteConfigAsIs = true
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	generated, err := BuildConfig(path, marshalCoreOptions(t, coreOptions))
	if err != nil {
		t.Fatalf("BuildConfig failed: %v", err)
	}

	if strings.Contains(generated, "fastly.jsdelivr.net") {
		t.Fatal("expected runtime config to avoid remote bundled rule-set urls")
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(generated), &decoded); err != nil {
		t.Fatalf("unmarshal generated config failed: %v", err)
	}

	certificate, _ := decoded["certificate"].(map[string]any)
	if runtime.GOOS == "android" {
		if certificate == nil || certificate["store"] != string(C.CertificateStoreMozilla) {
			t.Fatalf("expected Android runtime defaults to inject mozilla certificate store, got %#v", certificate)
		}
	} else if certificate != nil {
		t.Fatalf("expected non-android runtime to leave certificate defaults untouched, got %#v", certificate)
	}

	route, _ := decoded["route"].(map[string]any)
	if route == nil {
		t.Fatal("expected route options in generated config")
	}
	ruleSets, _ := route["rule_set"].([]any)
	if len(ruleSets) != 7 {
		t.Fatalf("expected localized bundled rule sets in generated config, got %d", len(ruleSets))
	}
	for _, raw := range ruleSets {
		ruleSet, _ := raw.(map[string]any)
		if ruleSet["type"] != string(C.RuleSetTypeLocal) {
			t.Fatalf("expected generated rule set to be local, got %#v", ruleSet)
		}
	}
}

func TestBuildConfig_OfficialSubscriptionFixtureProxyModeRewritesInbounds(t *testing.T) {
	t.Parallel()

	path := writeOfficialSubscriptionFixture(t)
	coreOptions := cfg.DefaultCoreOptions()
	coreOptions.ExecuteConfigAsIs = true
	coreOptions.EnableTun = false
	coreOptions.EnableTunService = false
	coreOptions.SetSystemProxy = true
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	generated, err := BuildConfig(path, marshalCoreOptions(t, coreOptions))
	if err != nil {
		t.Fatalf("BuildConfig failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(generated), &decoded); err != nil {
		t.Fatalf("unmarshal generated config failed: %v", err)
	}
	inbounds, _ := decoded["inbounds"].([]any)

	if runtime.GOOS == "android" {
		foundTun := false
		for _, raw := range inbounds {
			inbound, _ := raw.(map[string]any)
			if inbound["type"] == string(C.TypeTun) {
				foundTun = true
				break
			}
		}
		if !foundTun {
			t.Fatal("expected android runtime to keep tun inbound in proxy mode fixture")
		}
		return
	}

	if len(inbounds) < 2 {
		t.Fatalf("expected proxy runtime to expose mixed+dns inbounds, got %d", len(inbounds))
	}
	firstInbound, _ := inbounds[0].(map[string]any)
	if firstInbound["type"] != string(C.TypeMixed) {
		t.Fatalf("expected first inbound to be mixed, got %#v", firstInbound)
	}
	if firstInbound["set_system_proxy"] != true {
		t.Fatalf("expected proxy runtime to enable system proxy on mixed inbound, got %#v", firstInbound)
	}
	if _, ok := firstInbound["sniff"]; ok {
		t.Fatalf("expected proxy runtime mixed inbound to omit legacy sniff field, got %#v", firstInbound)
	}
	if _, ok := firstInbound["sniff_override_destination"]; ok {
		t.Fatalf("expected proxy runtime mixed inbound to omit legacy sniff_override_destination field, got %#v", firstInbound)
	}
	if _, ok := firstInbound["domain_strategy"]; ok {
		t.Fatalf("expected proxy runtime mixed inbound to omit legacy domain_strategy field, got %#v", firstInbound)
	}
	secondInbound, _ := inbounds[1].(map[string]any)
	if secondInbound["type"] != string(C.TypeDirect) {
		t.Fatalf("expected second inbound to be direct dns, got %#v", secondInbound)
	}
}

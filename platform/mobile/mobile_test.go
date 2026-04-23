package mobile

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	options, err := cfg.ParseOfficialOptions(generated)
	if err != nil {
		t.Fatalf("re-parse generated config failed: %v", err)
	}

	if options.Certificate == nil || options.Certificate.Store != C.CertificateStoreMozilla {
		t.Fatalf("expected Android runtime defaults to inject mozilla certificate store, got %#v", options.Certificate)
	}

	if options.Route == nil {
		t.Fatal("expected route options in generated config")
	}
	if len(options.Route.RuleSet) != 7 {
		t.Fatalf("expected localized bundled rule sets in generated config, got %d", len(options.Route.RuleSet))
	}
	for _, ruleSet := range options.Route.RuleSet {
		if ruleSet.Type != C.RuleSetTypeLocal {
			t.Fatalf("expected generated rule set %q to be local, got %q", ruleSet.Tag, ruleSet.Type)
		}
	}
}

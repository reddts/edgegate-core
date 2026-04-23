package config

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestNormalizeConfigForLegacySingBox_DNSTypedServers(t *testing.T) {
	input := []byte(`{
  "dns": {
    "servers": [
      {
        "type": "https",
        "tag": "dns-remote",
        "server": "1.1.1.1",
        "server_port": 443,
        "path": "/dns-query",
        "domain_resolver": "dns-local",
        "domain_strategy": "prefer_ipv4",
        "detour": "direct"
      },
      {
        "type": "udp",
        "tag": "dns-local",
        "server": "223.5.5.5"
      },
      {
        "type": "fakeip",
        "tag": "dns-fake"
      }
    ]
  }
}`)

	normalized, err := NormalizeConfigForLegacySingBox(input)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(normalized, &root); err != nil {
		t.Fatalf("normalized json invalid: %v", err)
	}
	dnsMap := root["dns"].(map[string]any)
	servers := dnsMap["servers"].([]any)

	first := servers[0].(map[string]any)
	if first["address"] != "https://1.1.1.1/dns-query" {
		t.Fatalf("unexpected https address: %v", first["address"])
	}
	if first["address_resolver"] != "dns-local" {
		t.Fatalf("domain_resolver not mapped, got: %v", first["address_resolver"])
	}
	if first["address_strategy"] != "prefer_ipv4" {
		t.Fatalf("domain_strategy not mapped, got: %v", first["address_strategy"])
	}
	if _, hasType := first["type"]; hasType {
		t.Fatalf("type field should be removed")
	}
	if _, hasServer := first["server"]; hasServer {
		t.Fatalf("server field should be removed")
	}

	second := servers[1].(map[string]any)
	if second["address"] != "223.5.5.5" {
		t.Fatalf("unexpected udp address: %v", second["address"])
	}

	third := servers[2].(map[string]any)
	if third["address"] != "fakeip" {
		t.Fatalf("unexpected fakeip address: %v", third["address"])
	}
}

func TestNormalizeConfigForLegacySingBox_DNSRuleActions(t *testing.T) {
	input := []byte(`{
  "dns": {
    "servers": [
      { "tag": "dns-main", "address": "223.5.5.5" }
    ],
    "rules": [
      {
        "domain_suffix": [".example.com"],
        "action": "route",
        "server": "dns-main"
      },
      {
        "domain_suffix": [".blocked.example"],
        "action": "reject"
      },
      {
        "domain_suffix": [".predefined.example"],
        "action": "predefined",
        "rcode": "refused"
      },
      {
        "domain_suffix": [".respond.example"],
        "action": "respond"
      }
    ]
  }
}`)

	normalized, err := NormalizeConfigForLegacySingBox(input)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(normalized, &root); err != nil {
		t.Fatalf("normalized json invalid: %v", err)
	}
	dnsMap := root["dns"].(map[string]any)

	// respond is not supported in legacy core and should be dropped.
	rules := dnsMap["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("unexpected rules len: %d", len(rules))
	}

	rejectRule := rules[1].(map[string]any)
	if rejectRule["server"] != "__compat_rcode_refused" {
		t.Fatalf("reject action not converted, got server: %v", rejectRule["server"])
	}
	if _, hasAction := rejectRule["action"]; hasAction {
		t.Fatalf("action field should be removed")
	}

	servers := dnsMap["servers"].([]any)
	foundCompatReject := false
	for _, server := range servers {
		serverMap := server.(map[string]any)
		if serverMap["tag"] == "__compat_rcode_refused" {
			foundCompatReject = true
			if serverMap["address"] != "rcode://refused" {
				t.Fatalf("unexpected compat server address: %v", serverMap["address"])
			}
		}
	}
	if !foundCompatReject {
		t.Fatal("compat reject DNS server not injected")
	}
}

func TestNormalizeConfigForLegacySingBox_RouteUnknownFields(t *testing.T) {
	input := []byte(`{
  "route": {
    "rules": [],
    "default_domain_resolver": "dns-remote",
    "network_strategy": "prefer_ipv4",
    "default_interface": "wlan0"
  }
}`)

	normalized, err := NormalizeConfigForLegacySingBox(input)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(normalized, &root); err != nil {
		t.Fatalf("normalized json invalid: %v", err)
	}
	routeMap, ok := root["route"].(map[string]any)
	if !ok {
		t.Fatalf("route missing in normalized config")
	}
	if _, ok := routeMap["default_domain_resolver"]; ok {
		t.Fatalf("default_domain_resolver should be removed for legacy route parser")
	}
	if _, ok := routeMap["network_strategy"]; ok {
		t.Fatalf("network_strategy should be removed for legacy route parser")
	}
	if routeMap["default_interface"] != "wlan0" {
		t.Fatalf("default_interface should be preserved, got: %v", routeMap["default_interface"])
	}
}

func TestNormalizeConfigForLegacySingBox_StripsOverrideAndroidVPNOnNonAndroid(t *testing.T) {
	input := []byte(`{
  "route": {
    "rules": [],
    "override_android_vpn": true
  }
}`)

	normalized, err := NormalizeConfigForLegacySingBox(input)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(normalized, &root); err != nil {
		t.Fatalf("normalized json invalid: %v", err)
	}
	routeMap, ok := root["route"].(map[string]any)
	if !ok {
		t.Fatalf("route missing in normalized config")
	}

	_, hasOverride := routeMap["override_android_vpn"]
	wantOverride := runtime.GOOS == "android"
	if hasOverride != wantOverride {
		t.Fatalf("expected override_android_vpn presence=%v on %s, got %v", wantOverride, runtime.GOOS, hasOverride)
	}
}

package config

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	SJ "github.com/sagernet/sing/common/json"
	dns "github.com/sagernet/sing-dns"
)

func TestBuildDNSServerRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"udp://1.1.1.1",
		"tcp://1.1.1.1",
		"tls://1.1.1.1",
		"https://1.1.1.1/dns-query",
		"quic://1.1.1.1",
		"local",
		"fakeip",
	}

	ctx := include.Context(context.Background())
	for _, address := range testCases {
		address := address
		t.Run(address, func(t *testing.T) {
			server, err := buildDNSServer(
				"test-dns",
				address,
				"dns-local",
				option.DomainStrategy(dns.DomainStrategyAsIS),
				"direct",
			)
			if err != nil {
				t.Fatalf("buildDNSServer failed: %v", err)
			}

			content, err := json.Marshal(server)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			var decoded option.DNSServerOptions
			if err := SJ.UnmarshalContext(ctx, content, &decoded); err != nil {
				t.Fatalf("round-trip unmarshal failed for %q: %v; json=%s", address, err, string(content))
			}
		})
	}
}

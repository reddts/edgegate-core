package config

import (
	"strings"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestToJsonPreservesTypedDNSServerFields(t *testing.T) {
	options := option.Options{
		DNS: &option.DNSOptions{
			RawDNSOptions: option.RawDNSOptions{
				Servers: []option.DNSServerOptions{
					{
						Type: C.DNSTypeQUIC,
						Tag:  "alidns",
						Options: &option.RemoteTLSDNSServerOptions{
							RemoteDNSServerOptions: option.RemoteDNSServerOptions{
								DNSServerAddressOptions: option.DNSServerAddressOptions{
									Server:     "223.6.6.6",
									ServerPort: 853,
								},
							},
							OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
								TLS: &option.OutboundTLSOptions{
									Enabled:  true,
									Insecure: true,
								},
							},
						},
					},
				},
			},
		},
	}

	content, err := ToJson(options)
	if err != nil {
		t.Fatalf("ToJson failed: %v", err)
	}
	for _, expected := range []string{
		`"server": "223.6.6.6"`,
		`"server_port": 853`,
		`"insecure": true`,
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("expected serialized config to contain %s, got %s", expected, content)
		}
	}
}

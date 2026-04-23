package config

import (
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
)

var legacyDNSServerAllowedFields = map[string]struct{}{
	"tag":                    {},
	"address":                {},
	"address_resolver":       {},
	"address_strategy":       {},
	"address_fallback_delay": {},
	"strategy":               {},
	"detour":                 {},
	"client_subnet":          {},
}

var legacyDNSRuleAllowedFields = map[string]struct{}{
	"type":                                   {},
	"mode":                                   {},
	"rules":                                  {},
	"invert":                                 {},
	"inbound":                                {},
	"ip_version":                             {},
	"query_type":                             {},
	"network":                                {},
	"auth_user":                              {},
	"protocol":                               {},
	"domain":                                 {},
	"domain_suffix":                          {},
	"domain_keyword":                         {},
	"domain_regex":                           {},
	"geosite":                                {},
	"source_geoip":                           {},
	"geoip":                                  {},
	"ip_cidr":                                {},
	"ip_is_private":                          {},
	"source_ip_cidr":                         {},
	"source_ip_is_private":                   {},
	"source_port":                            {},
	"source_port_range":                      {},
	"port":                                   {},
	"port_range":                             {},
	"process_name":                           {},
	"process_path":                           {},
	"process_path_regex":                     {},
	"package_name":                           {},
	"user":                                   {},
	"user_id":                                {},
	"outbound":                               {},
	"clash_mode":                             {},
	"wifi_ssid":                              {},
	"wifi_bssid":                             {},
	"rule_set":                               {},
	"rule_set_ipcidr_match_source":           {},
	"rule_set_ip_cidr_match_source":          {},
	"rule_set_ip_cidr_accept_empty":          {},
	"server":                                 {},
	"disable_cache":                          {},
	"rewrite_ttl":                            {},
	"client_subnet":                          {},
	"deprecated_ruleset_ipcidr_match_source": {},
}

var legacyRouteAllowedFields = map[string]struct{}{
	"geoip":                 {},
	"geosite":               {},
	"rules":                 {},
	"rule_set":              {},
	"final":                 {},
	"find_process":          {},
	"auto_detect_interface": {},
	"override_android_vpn":  {},
	"default_interface":     {},
	"default_mark":          {},
}

// NormalizeConfigForLegacySingBox normalizes known newer sing-box fields into
// legacy forms accepted by the currently pinned libbox parser.
func NormalizeConfigForLegacySingBox(content []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, err
	}

	dnsValue, ok := root["dns"]
	dnsMap, ok := dnsValue.(map[string]any)
	if !ok {
		dnsMap = nil
	}
	changed := false

	if dnsMap != nil {
		if serversValue, ok := dnsMap["servers"]; ok {
			if servers, ok := serversValue.([]any); ok {
				for i, serverValue := range servers {
					serverMap, ok := serverValue.(map[string]any)
					if !ok {
						continue
					}
					if normalizeDNSServerCompat(serverMap) {
						servers[i] = serverMap
						changed = true
					}
				}
				dnsMap["servers"] = servers
			}
		}

		if rulesValue, ok := dnsMap["rules"]; ok {
			if rules, ok := rulesValue.([]any); ok {
				normalizedRules := make([]any, 0, len(rules))
				for _, ruleValue := range rules {
					ruleMap, ok := ruleValue.(map[string]any)
					if !ok {
						continue
					}
					keep, ruleChanged := normalizeDNSRuleCompat(ruleMap, dnsMap)
					if ruleChanged {
						changed = true
					}
					if keep {
						normalizedRules = append(normalizedRules, ruleMap)
					} else {
						changed = true
					}
				}
				dnsMap["rules"] = normalizedRules
				changed = true
			}
		}
	}

	if routeValue, ok := root["route"]; ok {
		if routeMap, ok := routeValue.(map[string]any); ok {
			if normalizeRouteCompat(routeMap) {
				root["route"] = routeMap
				changed = true
			}
		}
	}

	if !changed {
		return content, nil
	}
	if dnsMap != nil {
		root["dns"] = dnsMap
	}
	return json.Marshal(root)
}

func normalizeRouteCompat(route map[string]any) bool {
	changed := false
	if runtime.GOOS != "android" {
		if _, ok := route["override_android_vpn"]; ok {
			delete(route, "override_android_vpn")
			changed = true
		}
	}
	for key := range route {
		if _, ok := legacyRouteAllowedFields[key]; !ok {
			delete(route, key)
			changed = true
		}
	}
	return changed
}

func normalizeDNSServerCompat(server map[string]any) bool {
	changed := false

	// Map newer aliases to legacy fields.
	if v, ok := server["domain_resolver"]; ok && server["address_resolver"] == nil {
		server["address_resolver"] = v
		changed = true
	}
	if v, ok := server["domain_strategy"]; ok && server["address_strategy"] == nil {
		server["address_strategy"] = v
		changed = true
	}

	if address := stringField(server, "address"); address == "" {
		if dnsType := strings.ToLower(stringField(server, "type")); dnsType != "" {
			if normalizedAddress, ok := dnsAddressFromTypedServer(dnsType, server); ok {
				server["address"] = normalizedAddress
				changed = true
			}
		}
	}

	// Drop fields unsupported by legacy DNSServerOptions, otherwise
	// DisallowUnknownFields() will fail.
	for key := range server {
		if _, ok := legacyDNSServerAllowedFields[key]; !ok {
			delete(server, key)
			changed = true
		}
	}

	return changed
}

func normalizeDNSRuleCompat(rule map[string]any, dnsMap map[string]any) (bool, bool) {
	changed := false

	if ruleType := strings.ToLower(stringField(rule, "type")); ruleType == "logical" {
		nestedRules, ok := rule["rules"].([]any)
		if ok {
			normalizedNestedRules := make([]any, 0, len(nestedRules))
			for _, nestedRuleValue := range nestedRules {
				nestedRuleMap, ok := nestedRuleValue.(map[string]any)
				if !ok {
					continue
				}
				keep, nestedChanged := normalizeDNSRuleCompat(nestedRuleMap, dnsMap)
				if nestedChanged {
					changed = true
				}
				if keep {
					normalizedNestedRules = append(normalizedNestedRules, nestedRuleMap)
				} else {
					changed = true
				}
			}
			rule["rules"] = normalizedNestedRules
			if len(normalizedNestedRules) == 0 {
				return false, true
			}
		}
	}

	action := strings.ToLower(stringField(rule, "action"))
	if action != "" {
		delete(rule, "action")
		changed = true
		switch action {
		case "route":
			// route is legacy behavior, keep server/options fields.
		case "evaluate":
			// evaluate has no legacy equivalent, route to target server.
		case "route-options":
			// Keep mutable DNS route options only.
			delete(rule, "server")
		case "reject":
			ensureCompatRCodeServer(rule, dnsMap, "refused")
		case "predefined":
			rcode := stringField(rule, "rcode")
			if rcode == "" {
				rcode = "success"
			}
			ensureCompatRCodeServer(rule, dnsMap, rcode)
		case "respond":
			// respond depends on evaluate cache semantics unavailable in legacy core.
			// Drop this rule to avoid parse failure and invalid legacy semantics.
			return false, true
		default:
			// Unknown action in new schema: drop rule conservatively.
			return false, true
		}
	}

	// Newer schema aliases.
	if _, ok := rule["rule_set_ipcidr_match_source"]; ok {
		if _, exists := rule["rule_set_ip_cidr_match_source"]; !exists {
			rule["rule_set_ip_cidr_match_source"] = rule["rule_set_ipcidr_match_source"]
			changed = true
		}
	}

	// Strip unsupported keys for legacy strict decoder.
	for key := range rule {
		if _, ok := legacyDNSRuleAllowedFields[key]; !ok {
			delete(rule, key)
			changed = true
		}
	}

	// Legacy default rule must have at least one matcher or action fields.
	if strings.ToLower(stringField(rule, "type")) != "logical" {
		if !hasAnyDNSRuleMatcherOrAction(rule) {
			return false, true
		}
	}

	return true, changed
}

func hasAnyDNSRuleMatcherOrAction(rule map[string]any) bool {
	keys := []string{
		"inbound", "ip_version", "query_type", "network", "auth_user", "protocol",
		"domain", "domain_suffix", "domain_keyword", "domain_regex", "geosite",
		"source_geoip", "geoip", "ip_cidr", "ip_is_private", "source_ip_cidr",
		"source_ip_is_private", "source_port", "source_port_range", "port",
		"port_range", "process_name", "process_path", "process_path_regex",
		"package_name", "user", "user_id", "outbound", "clash_mode", "wifi_ssid",
		"wifi_bssid", "rule_set", "rule_set_ip_cidr_match_source",
		"rule_set_ipcidr_match_source", "rule_set_ip_cidr_accept_empty", "server",
		"disable_cache", "rewrite_ttl", "client_subnet", "invert",
	}
	for _, key := range keys {
		if value, ok := rule[key]; ok && value != nil {
			switch v := value.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return true
				}
			case []any:
				if len(v) > 0 {
					return true
				}
			case bool:
				if v {
					return true
				}
			default:
				return true
			}
		}
	}
	return false
}

func ensureCompatRCodeServer(rule map[string]any, dnsMap map[string]any, rcode string) {
	normalizedCode := strings.ToLower(strings.TrimSpace(rcode))
	if normalizedCode == "" {
		normalizedCode = "refused"
	}
	tag := "__compat_rcode_" + normalizedCode
	ensureCompatDNSServer(dnsMap, tag, "rcode://"+normalizedCode)
	rule["server"] = tag
	delete(rule, "rcode")
	delete(rule, "answer")
	delete(rule, "ns")
	delete(rule, "extra")
	delete(rule, "method")
	delete(rule, "no_drop")
}

func ensureCompatDNSServer(dnsMap map[string]any, tag string, address string) {
	serversValue, ok := dnsMap["servers"]
	if !ok {
		serversValue = []any{}
	}
	servers, ok := serversValue.([]any)
	if !ok {
		servers = []any{}
	}
	for _, serverValue := range servers {
		serverMap, ok := serverValue.(map[string]any)
		if !ok {
			continue
		}
		if stringField(serverMap, "tag") == tag {
			return
		}
	}
	servers = append(servers, map[string]any{
		"tag":     tag,
		"address": address,
	})
	dnsMap["servers"] = servers
}

func dnsAddressFromTypedServer(dnsType string, server map[string]any) (string, bool) {
	host := stringField(server, "server")
	port := intField(server, "server_port")
	path := stringField(server, "path")

	switch dnsType {
	case "local":
		return "local", true
	case "fakeip":
		return "fakeip", true
	case "dhcp":
		iface := stringField(server, "interface")
		if iface == "" {
			iface = "auto"
		}
		return "dhcp://" + iface, true
	case "rcode":
		code := stringField(server, "rcode")
		if code == "" {
			code = stringField(server, "code")
		}
		if code == "" {
			code = stringField(server, "response_code")
		}
		if code == "" {
			code = "refused"
		}
		return "rcode://" + code, true
	case "udp":
		if host == "" {
			return "", false
		}
		return formatAddressHost(host, port, 53), true
	case "tcp":
		if host == "" {
			return "", false
		}
		return "tcp://" + formatAddressHost(host, port, 53), true
	case "tls":
		if host == "" {
			return "", false
		}
		return "tls://" + formatAddressHost(host, port, 853), true
	case "https":
		if host == "" {
			return "", false
		}
		return "https://" + formatAddressHost(host, port, 443) + normalizePath(path), true
	case "h3", "http3":
		if host == "" {
			return "", false
		}
		return "h3://" + formatAddressHost(host, port, 443) + normalizePath(path), true
	case "quic":
		if host == "" {
			return "", false
		}
		return "quic://" + formatAddressHost(host, port, 853), true
	default:
		return "", false
	}
}

func stringField(m map[string]any, key string) string {
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func intField(m map[string]any, key string) int {
	value, ok := m[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(v))
		return i
	default:
		i, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
		return i
	}
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/dns-query"
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}

func formatAddressHost(host string, port int, defaultPort int) string {
	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		return ""
	}
	if hasExplicitPort(trimmedHost) {
		return trimmedHost
	}
	if port <= 0 {
		port = defaultPort
	}
	if port == defaultPort {
		return trimmedHost
	}
	return net.JoinHostPort(trimmedHost, strconv.Itoa(port))
}

func hasExplicitPort(host string) bool {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return true
	}
	if strings.Count(host, ":") == 1 && !strings.Contains(host, "]") {
		idx := strings.LastIndex(host, ":")
		if idx > 0 {
			_, err := strconv.Atoi(host[idx+1:])
			return err == nil
		}
	}
	return false
}

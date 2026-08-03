package store

import (
	"encoding/json"
	"testing"
)

func TestMergeAgentRoutingSettingsIncludesRulesAndOutboundChain(t *testing.T) {
	settings := json.RawMessage(`{"transport":"tcp","custom_outbounds":[{"tag":"manual","protocol":"socks","settings":{"server":"127.0.0.1"}}]}`)
	rules := json.RawMessage(`[{"name":"media","match":{"domain_suffixes":["example.com"]},"action":{"type":"route","target":"media-out"}}]`)
	outbounds := json.RawMessage(`[
		{"tag":"warp","protocol":"wireguard","settings":{"private_key":"key"}},
		{"tag":"unused","protocol":"socks","settings":{"server":"10.0.0.2"}},
		{"tag":"media-out","protocol":"socks","proxy_tag":"warp","settings":{"server":"10.0.0.1"}}
	]`)

	merged, err := mergeAgentRoutingSettings(settings, rules, outbounds, true)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Transport       string           `json:"transport"`
		CustomRules     []map[string]any `json:"custom_route_rules"`
		CustomOutbounds []map[string]any `json:"custom_outbounds"`
	}
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Transport != "tcp" || len(decoded.CustomRules) != 1 {
		t.Fatalf("node settings or rules were lost: %s", merged)
	}
	if len(decoded.CustomOutbounds) != 3 {
		t.Fatalf("expected manual outbound and two managed chain entries, got %s", merged)
	}
	tags := make(map[string]bool)
	for _, outbound := range decoded.CustomOutbounds {
		tags[outbound["tag"].(string)] = true
	}
	if !tags["manual"] || !tags["warp"] || !tags["media-out"] || tags["unused"] {
		t.Fatalf("unexpected outbound selection: %#v", tags)
	}
}

func TestMergeAgentRoutingSettingsClearsRulesForDisabledPolicy(t *testing.T) {
	merged, err := mergeAgentRoutingSettings(json.RawMessage(`{"transport":"tcp"}`), json.RawMessage(`[{"match":{"ports":["443"]},"action":{"type":"block"}}]`), json.RawMessage(`[]`), false)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatal(err)
	}
	if rules, ok := decoded["custom_route_rules"].([]any); !ok || len(rules) != 0 {
		t.Fatalf("disabled policy should emit an empty rule set: %s", merged)
	}
}

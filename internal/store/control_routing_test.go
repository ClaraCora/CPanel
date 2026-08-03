package store

import (
	"encoding/json"
	"testing"

	"cpanel/internal/domain"
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

func TestMergeAgentRoutingSettingsCompilesDomainRegexForXray(t *testing.T) {
	rules := json.RawMessage(`[{"name":"blocked","match":{"domain_regexes":["^(.+\\.)?example\\.com$"]},"action":{"type":"block"}}]`)
	merged, err := mergeAgentRoutingSettings(json.RawMessage(`{}`), rules, json.RawMessage(`[]`), true)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Rules []struct {
			Match map[string][]string `json:"match"`
		} `json:"custom_route_rules"`
	}
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Rules) != 1 || len(decoded.Rules[0].Match["domains"]) != 1 || decoded.Rules[0].Match["domains"][0] != `regexp:^(.+\.)?example\.com$` {
		t.Fatalf("domain regex was not compiled for Agent: %s", merged)
	}
	if _, exists := decoded.Rules[0].Match["domain_regexes"]; exists {
		t.Fatalf("Agent payload must not contain panel-only domain_regexes: %s", merged)
	}
}

func TestOutboundUpdateAffectsNodeSpecs(t *testing.T) {
	name := "display only"
	settings := json.RawMessage(`{"servers":[{"address":"127.0.0.1","port":1080}]}`)
	if outboundUpdateAffectsNodeSpecs(domain.OutboundUpdate{Name: &name}) {
		t.Fatal("renaming an outbound should not replace every published node spec")
	}
	if !outboundUpdateAffectsNodeSpecs(domain.OutboundUpdate{Settings: &settings}) {
		t.Fatal("changing outbound settings must replace published node specs")
	}
}

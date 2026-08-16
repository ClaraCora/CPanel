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

	merged, err := mergeAgentRoutingSettings(settings, rules, outbounds, "", true)
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
	merged, err := mergeAgentRoutingSettings(json.RawMessage(`{"transport":"tcp"}`), json.RawMessage(`[{"match":{"ports":["443"]},"action":{"type":"block"}}]`), json.RawMessage(`[]`), "warp", false)
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
	if value, _ := decoded["default_outbound_tag"].(string); value != "direct" {
		t.Fatalf("disabled policy should fall back to direct: %s", merged)
	}
}

func TestMergeAgentRoutingSettingsDefaultsToDirect(t *testing.T) {
	merged, err := mergeAgentRoutingSettings(json.RawMessage(`{"default_outbound_tag":"warp"}`), json.RawMessage(`[]`), json.RawMessage(`[]`), "", true)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatal(err)
	}
	if value, _ := decoded["default_outbound_tag"].(string); value != "direct" {
		t.Fatalf("empty policy default = %q, want direct", value)
	}
}

func TestMergeAgentRoutingProfilesDefaultsToDirect(t *testing.T) {
	merged, err := mergeAgentRoutingProfiles(json.RawMessage(`{"default_outbound_tag":"warp"}`), map[string]map[string]any{
		"default": emptyAgentRouteProfile(),
		"admin":   emptyAgentRouteProfile(),
		"member":  emptyAgentRouteProfile(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		DefaultOutboundTag string `json:"default_outbound_tag"`
	}
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.DefaultOutboundTag != "direct" {
		t.Fatalf("scoped empty policy default = %q, want direct", decoded.DefaultOutboundTag)
	}
}

func TestMergeAgentRoutingSettingsCompilesDomainRegexForXray(t *testing.T) {
	rules := json.RawMessage(`[{"name":"blocked","match":{"domain_regexes":["^(.+\\.)?example\\.com$"]},"action":{"type":"block"}}]`)
	merged, err := mergeAgentRoutingSettings(json.RawMessage(`{}`), rules, json.RawMessage(`[]`), "", true)
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

func TestMergeAgentRoutingSettingsPreservesGeoIPCategories(t *testing.T) {
	rules := json.RawMessage(`[{"name":"google","match":{"geo_ips":["google"]},"action":{"type":"route","target":"landing"}}]`)
	outbounds := json.RawMessage(`[{"tag":"landing","protocol":"socks","settings":{"server":"127.0.0.1","server_port":1080}}]`)
	merged, err := mergeAgentRoutingSettings(json.RawMessage(`{}`), rules, outbounds, "landing", true)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Rules []struct {
			Match domain.RoutePolicyMatch `json:"match"`
		} `json:"custom_route_rules"`
	}
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Rules) != 1 || len(decoded.Rules[0].Match.GeoIPs) != 1 || decoded.Rules[0].Match.GeoIPs[0] != "google" {
		t.Fatalf("GeoIP categories were not preserved for Agent: %s", merged)
	}
}

func TestMergeAgentRoutingSettingsIncludesDefaultOutbound(t *testing.T) {
	outbounds := json.RawMessage(`[
		{"tag":"warp","protocol":"wireguard","settings":{"private_key":"key"}},
		{"tag":"unused","protocol":"socks","settings":{"server":"10.0.0.2"}}
	]`)
	merged, err := mergeAgentRoutingSettings(json.RawMessage(`{"transport":"tcp"}`), json.RawMessage(`[]`), outbounds, "warp", true)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		DefaultOutboundTag string           `json:"default_outbound_tag"`
		CustomOutbounds    []map[string]any `json:"custom_outbounds"`
	}
	if err := json.Unmarshal(merged, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.DefaultOutboundTag != "warp" {
		t.Fatalf("default outbound = %q, want warp", decoded.DefaultOutboundTag)
	}
	if len(decoded.CustomOutbounds) != 1 || decoded.CustomOutbounds[0]["tag"] != "warp" {
		t.Fatalf("default outbound was not selected: %s", merged)
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

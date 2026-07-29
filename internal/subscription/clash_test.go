package subscription

import (
	"encoding/json"
	"strings"
	"testing"

	"cpanel/internal/domain"
)

func TestBuildClashMeta(t *testing.T) {
	result, err := BuildClashMeta(domain.Subscription{Nodes: []domain.SubscriptionNode{{
		Name: "HK-01", Host: "hk.example.com", Port: 443, Protocol: "vless",
		UserUUID:   "11111111-1111-4111-8111-111111111111",
		NodeConfig: json.RawMessage(`{"tls":1,"server_name":"hk.example.com"}`),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result)
	for _, expected := range []string{"name: HK-01", "type: vless", "servername: hk.example.com", "MATCH,CPanel"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("subscription missing %q:\n%s", expected, text)
		}
	}
}

func TestBuildClashMetaIncludesRealityClientSettings(t *testing.T) {
	result, err := BuildClashMeta(domain.Subscription{Nodes: []domain.SubscriptionNode{{
		Name: "Reality-01", Host: "reality.example.com", Port: 443, Protocol: "vless",
		UserUUID: "11111111-1111-4111-8111-111111111111",
		NodeConfig: json.RawMessage(`{
			"transport":"tcp",
			"flow":"xtls-rprx-vision",
			"tls":{
				"enabled":2,
				"public_key":"q1W2e3R4t5Y6u7I8o9P0a1S2d3F4g5H6j7K8l9Z0x1Q",
				"short_id":"6ba85179e30d4fc2",
				"server_name":"www.example.com",
				"fingerprint":"chrome"
			}
		}`),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result)
	for _, expected := range []string{
		"tls: true",
		"servername: www.example.com",
		"client-fingerprint: chrome",
		"flow: xtls-rprx-vision",
		"network: tcp",
		"reality-opts:",
		"public-key: q1W2e3R4t5Y6u7I8o9P0a1S2d3F4g5H6j7K8l9Z0x1Q",
		"short-id: 6ba85179e30d4fc2",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("reality subscription missing %q:\n%s", expected, text)
		}
	}
}

package subscription

import (
	"encoding/base64"
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

func TestProxyForNodeBuildsSS2022Password(t *testing.T) {
	tests := []struct {
		name   string
		cipher string
		size   int
	}{
		{name: "aes-128", cipher: "2022-blake3-aes-128-gcm", size: 16},
		{name: "aes-256", cipher: "2022-blake3-aes-256-gcm", size: 32},
	}
	const udid = "279d4f89-3a2c-488d-a67c-2d39a72acdde"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serverRaw := []byte(strings.Repeat("s", test.size))
			serverKey := base64.StdEncoding.EncodeToString(serverRaw)
			config, err := json.Marshal(map[string]any{
				"cipher":     test.cipher,
				"server_key": serverKey,
			})
			if err != nil {
				t.Fatal(err)
			}
			proxy, err := proxyForNode(domain.SubscriptionNode{
				Name: "SS2022", Host: "ss.example.com", Port: 8388, Protocol: "shadowsocks",
				UserUUID: udid, NodeConfig: config,
			})
			if err != nil {
				t.Fatal(err)
			}
			userKey := base64.StdEncoding.EncodeToString([]byte(udid[:test.size]))
			want := serverKey + ":" + userKey
			if got := proxy["password"]; got != want {
				t.Fatalf("password = %q, want %q", got, want)
			}
		})
	}
}

func TestProxyForNodeKeepsTraditionalShadowsocksPassword(t *testing.T) {
	proxy, err := proxyForNode(domain.SubscriptionNode{
		Name: "SS", Host: "ss.example.com", Port: 8388, Protocol: "shadowsocks",
		UserUUID: "user-udid", NodeConfig: json.RawMessage(`{"cipher":"aes-128-gcm"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := proxy["password"]; got != "user-udid" {
		t.Fatalf("password = %q, want user-udid", got)
	}
}

func TestProxyForNodeUsesEntryNameForMultiEntryNode(t *testing.T) {
	proxy, err := proxyForNode(domain.SubscriptionNode{
		Name: "香港-绿云", EntryName: "香港-绿云[广港HS]", Host: "198.51.100.7", Port: 443,
		Protocol: "vless", UserUUID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := proxy["name"]; got != "香港-绿云[广港HS]" {
		t.Fatalf("proxy name = %q, want entry-specific name", got)
	}
}

func TestBuildClashMetaKeepsMultiEntryNamesDistinct(t *testing.T) {
	base := func(entry, host string) domain.SubscriptionNode {
		return domain.SubscriptionNode{
			Name: "香港-绿云", EntryName: entry, Host: host, Port: 443,
			Protocol: "vless", UserUUID: "11111111-1111-4111-8111-111111111111",
		}
	}
	result, err := BuildClashMeta(domain.Subscription{Nodes: []domain.SubscriptionNode{
		base("香港-绿云[广港HS]", "198.51.100.7"),
		base("香港-绿云[广港PO0]", "198.51.100.8"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(result)
	for _, name := range []string{"香港-绿云[广港HS]", "香港-绿云[广港PO0]"} {
		if !strings.Contains(text, name) {
			t.Fatalf("subscription missing entry-specific name %q:\n%s", name, text)
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

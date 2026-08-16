package subscription

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"cpanel/internal/domain"
)

func TestBuildNodeURIVLESSReality(t *testing.T) {
	uri, err := BuildNodeURI(domain.SubscriptionNode{
		Name: "JP Reality", EntryName: "东京入口", Host: "jp.example.com", Port: 443, Protocol: "vless",
		UserUUID: "550e8400-e29b-41d4-a716-446655440000",
		NodeConfig: json.RawMessage(`{
			"transport":"tcp", "flow":"xtls-rprx-vision",
			"tls":{"enabled":2,"server_name":"www.example.com","fingerprint":"chrome","public_key":"public-key","short_id":"a1b2"}
		}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "vless" || parsed.User.Username() != "550e8400-e29b-41d4-a716-446655440000" || parsed.Host != "jp.example.com:443" {
		t.Fatalf("unexpected VLESS target: %s", uri)
	}
	query := parsed.Query()
	for key, want := range map[string]string{"encryption": "none", "type": "tcp", "security": "reality", "pbk": "public-key", "sid": "a1b2", "sni": "www.example.com", "fp": "chrome", "flow": "xtls-rprx-vision"} {
		if got := query.Get(key); got != want {
			t.Errorf("query[%q] = %q, want %q", key, got, want)
		}
	}
	if parsed.Fragment != "JP Reality · 东京入口" {
		t.Errorf("fragment = %q", parsed.Fragment)
	}
}

func TestDisplayNameDoesNotDuplicateNodePrefix(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		want  string
	}{
		{name: "香港-绿云", entry: "香港-绿云[广港HS]", want: "香港-绿云[广港HS]"},
		{name: "JP Reality", entry: "东京入口", want: "JP Reality · 东京入口"},
		{name: "SS", entry: "SS", want: "SS"},
	}
	for _, test := range tests {
		t.Run(test.entry, func(t *testing.T) {
			got := displayName(domain.SubscriptionNode{Name: test.name, EntryName: test.entry})
			if got != test.want {
				t.Fatalf("display name = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBuildNodeURIShadowsocks2022(t *testing.T) {
	serverKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("s", 16)))
	uri, err := BuildNodeURI(domain.SubscriptionNode{
		Name: "SS 2022", Host: "198.51.100.7", Port: 8388, Protocol: "shadowsocks", UserUUID: "user-udid",
		NodeConfig: json.RawMessage(`{"cipher":"2022-blake3-aes-128-gcm","server_key":"` + serverKey + `"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "ss" || parsed.Host != "198.51.100.7:8388" {
		t.Fatalf("unexpected Shadowsocks target: %s", uri)
	}
	credentials, err := base64.RawURLEncoding.DecodeString(parsed.User.Username())
	if err != nil {
		t.Fatal(err)
	}
	userKey, _ := ss2022UserKey("2022-blake3-aes-128-gcm", "user-udid")
	want := "2022-blake3-aes-128-gcm:" + serverKey + ":" + userKey
	if got := string(credentials); got != want {
		t.Fatalf("credentials = %q, want %q", got, want)
	}
}

func TestBuildNodeURIVMess(t *testing.T) {
	uri, err := BuildNodeURI(domain.SubscriptionNode{
		Name: "VMess", Host: "vm.example.com", Port: 8443, Protocol: "vmess", UserUUID: "550e8400-e29b-41d4-a716-446655440000",
		NodeConfig: json.RawMessage(`{"transport":"ws","network_settings":{"path":"/socket","host":"cdn.example.com"},"tls":{"enabled":true,"server_name":"sni.example.com","fingerprint":"firefox"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uri, "vmess://") {
		t.Fatalf("unexpected VMess URI: %s", uri)
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(uri, "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err := json.Unmarshal(payload, &values); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"add": "vm.example.com", "port": "8443", "net": "ws", "path": "/socket", "host": "cdn.example.com", "tls": "tls", "sni": "sni.example.com", "fp": "firefox"} {
		if got := values[key]; got != want {
			t.Errorf("payload[%q] = %q, want %q", key, got, want)
		}
	}
}

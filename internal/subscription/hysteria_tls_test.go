package subscription

import (
	"encoding/json"
	"net/url"
	"testing"

	"cpanel/internal/domain"
)

func TestHysteria2CertificateVerificationSubscriptions(t *testing.T) {
	for _, test := range []struct {
		name, config string
		insecure     bool
	}{
		{"explicit skip", `{"version":2,"tls":{"enabled":true,"server_name":"edge.example.com","insecure":true},"cert_config":{"cert_mode":"self"}}`, true},
		{"explicit verification", `{"version":2,"tls":{"enabled":true,"server_name":"edge.example.com","insecure":false},"cert_config":{"cert_mode":"self"}}`, false},
		{"legacy self signed", `{"version":2,"tls":{"enabled":true,"server_name":"edge.example.com"},"cert_config":{"cert_mode":"self"}}`, true},
		{"CA certificate", `{"version":2,"tls":{"enabled":true,"server_name":"edge.example.com"},"cert_config":{"cert_mode":"http"}}`, false},
		{"no certificate settings", `{"version":2,"tls":{"enabled":true,"server_name":"edge.example.com"}}`, false},
	} {
		for _, protocol := range []string{"hysteria", "hysteria2"} {
			t.Run(test.name+"/"+protocol, func(t *testing.T) {
				node := domain.SubscriptionNode{Name: "HY2", Host: "node.example.com", Port: 443,
					Protocol: protocol, UserUUID: "uuid", NodeConfig: json.RawMessage(test.config)}
				proxy, err := proxyForNode(node)
				if err != nil {
					t.Fatal(err)
				}
				if proxy["skip-cert-verify"] != test.insecure || proxy["servername"] != "edge.example.com" {
					t.Fatalf("unexpected Clash TLS options: %#v", proxy)
				}
				uri, err := BuildNodeURI(node)
				if err != nil {
					t.Fatal(err)
				}
				parsed, err := url.Parse(uri)
				if err != nil {
					t.Fatal(err)
				}
				if (parsed.Query().Get("insecure") == "1") != test.insecure || parsed.Query().Get("sni") != "edge.example.com" {
					t.Fatalf("unexpected Hysteria2 URI: %s", uri)
				}
			})
		}
	}
}

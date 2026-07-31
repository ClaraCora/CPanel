package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConventionalBrowserDetectionAndWhitelist(t *testing.T) {
	browser := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/138.0.0.0 Safari/537.36"
	if !isConventionalBrowserUA(browser) {
		t.Fatal("expected Chrome user agent to be detected as a browser")
	}
	if isConventionalBrowserUA("ClashMetaForAndroid/2.11.13.Meta") {
		t.Fatal("subscription client was detected as a browser")
	}
	if !uaWhitelistMatches(browser, "Clash\nChrome/138") {
		t.Fatal("expected case-insensitive substring whitelist to match")
	}
}

func TestNormalizeUAWhitelist(t *testing.T) {
	got, err := normalizeUAWhitelist(" Clash \r\nclash\r\nShadowrocket\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Clash\nShadowrocket" {
		t.Fatalf("normalized whitelist = %q", got)
	}
	if _, err := normalizeUAWhitelist(strings.Repeat("x", 257)); err == nil {
		t.Fatal("expected an overlong whitelist entry to fail")
	}
}

func TestResolvedClientIPUsesTrustedProxyChain(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "https://panel.example/ca/x/token", nil)
	request.RemoteAddr = "127.0.0.1:8256"
	request.Header.Set("X-Forwarded-For", "198.51.100.99, 203.0.113.8")
	if got := resolvedClientIP(request, ""); got != "203.0.113.8" {
		t.Fatalf("resolvedClientIP() = %q, want direct untrusted client", got)
	}

	request.RemoteAddr = "192.0.2.10:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.99")
	if got := resolvedClientIP(request, ""); got != "192.0.2.10" {
		t.Fatalf("untrusted proxy result = %q, want remote address", got)
	}

	request.RemoteAddr = "10.0.0.2:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.99, 10.0.0.3")
	if got := resolvedClientIP(request, "10.0.0.0/8"); got != "198.51.100.99" {
		t.Fatalf("trusted chain result = %q, want original client", got)
	}
}

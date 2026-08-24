package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"cpanel/internal/config"
)

func TestSessionTTLUsesConfiguredFallbackAndBounds(t *testing.T) {
	server := &Server{cfg: config.Config{SessionTTL: 12 * time.Hour}}
	if got := server.sessionTTL(context.Background()); got != 12*time.Hour {
		t.Fatalf("sessionTTL() = %s, want 12h", got)
	}
	server.cfg.SessionTTL = 90 * 24 * time.Hour
	if got := server.sessionTTL(context.Background()); got != 30*24*time.Hour {
		t.Fatalf("sessionTTL() = %s, want 30d cap", got)
	}
}

func TestSecureCookiesForHTTPSExternalURL(t *testing.T) {
	server := &Server{cfg: config.Config{ExternalURL: "https://panel.example.com"}}
	if !server.secureCookies(context.Background()) {
		t.Fatal("HTTPS external URL did not enable Secure cookies")
	}
	server.cfg.ExternalURL = "http://127.0.0.1:8256"
	if server.secureCookies(context.Background()) {
		t.Fatal("HTTP external URL unexpectedly enabled Secure cookies")
	}
	server.cfg.CookieSecure = true
	if !server.secureCookies(context.Background()) {
		t.Fatal("explicit CookieSecure setting was ignored")
	}
}

func TestLoginInputBounds(t *testing.T) {
	if !validAdminLogin("admin@example.com") || validAdminLogin("") || validAdminLogin(strings.Repeat("a", 255)) {
		t.Fatal("admin login bounds are incorrect")
	}
	if !validLoginPassword("password") || validLoginPassword("") || validLoginPassword(strings.Repeat("密", maximumLoginPasswordRunes+1)) {
		t.Fatal("password bounds are incorrect")
	}
}

func TestSensitiveSettingsAreServerDefined(t *testing.T) {
	for _, item := range []struct{ section, key string }{
		{"agent", "communication_key"},
		{"certificate", "dns_api_token"},
		{"tgbot", "bot_token"},
	} {
		if !isSensitiveSetting(item.section, item.key) {
			t.Errorf("%s.%s was not classified as sensitive", item.section, item.key)
		}
	}
	if isSensitiveSetting("site", "platform_name") || isSensitiveSetting("agent", "external_url") {
		t.Fatal("non-sensitive setting was classified as sensitive")
	}
}

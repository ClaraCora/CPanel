package config

import "testing"

func TestLoadUsesLoopbackDefaults(t *testing.T) {
	t.Setenv("CPANEL_ADDR", "")
	t.Setenv("CPANEL_EXTERNAL_URL", "")
	t.Setenv("CPANEL_DATABASE_URL", "postgres://example")
	t.Setenv("CPANEL_ENCRYPTION_KEY", "test-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != "127.0.0.1:8256" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, "127.0.0.1:8256")
	}
	if cfg.ExternalURL != "http://127.0.0.1:8256" {
		t.Fatalf("ExternalURL = %q, want %q", cfg.ExternalURL, "http://127.0.0.1:8256")
	}
}

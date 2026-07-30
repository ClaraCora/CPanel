package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                 string
	DatabaseURL          string
	ExternalURL          string
	EncryptionKey        string
	CookieSecure         bool
	SessionTTL           time.Duration
	HeartbeatInterval    time.Duration
	TelemetryInterval    time.Duration
	FallbackPullInterval time.Duration
	AgentArtifactDir     string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:                 env("CPANEL_ADDR", "127.0.0.1:8256"),
		DatabaseURL:          strings.TrimSpace(os.Getenv("CPANEL_DATABASE_URL")),
		ExternalURL:          strings.TrimRight(env("CPANEL_EXTERNAL_URL", "http://127.0.0.1:8256"), "/"),
		EncryptionKey:        strings.TrimSpace(os.Getenv("CPANEL_ENCRYPTION_KEY")),
		CookieSecure:         envBool("CPANEL_COOKIE_SECURE", false),
		SessionTTL:           envDuration("CPANEL_SESSION_TTL", 24*time.Hour),
		HeartbeatInterval:    envDuration("CPANEL_AGENT_HEARTBEAT_INTERVAL", 60*time.Second),
		TelemetryInterval:    envDuration("CPANEL_AGENT_TELEMETRY_INTERVAL", 60*time.Second),
		FallbackPullInterval: envDuration("CPANEL_AGENT_FALLBACK_PULL_INTERVAL", 60*time.Second),
		AgentArtifactDir:     env("CPANEL_AGENT_ARTIFACT_DIR", "/var/lib/cpanel/agent-artifacts"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("CPANEL_DATABASE_URL is required")
	}
	if cfg.EncryptionKey == "" {
		return Config{}, fmt.Errorf("CPANEL_ENCRYPTION_KEY is required")
	}
	if cfg.SessionTTL < time.Hour {
		return Config{}, fmt.Errorf("CPANEL_SESSION_TTL must be at least 1h")
	}
	if cfg.HeartbeatInterval < 5*time.Second {
		return Config{}, fmt.Errorf("CPANEL_AGENT_HEARTBEAT_INTERVAL must be at least 5s")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

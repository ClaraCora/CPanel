package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentReleaseResolverReturnsAndCachesShortVersion(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "v2.0.0\n")
	}))
	defer server.Close()

	resolver := newAgentReleaseResolver(server.Client(), server.URL)
	if version := resolver.Latest(context.Background()); version != "v2.0.0" {
		t.Fatalf("version = %q, want v2.0.0", version)
	}
	if version := resolver.Latest(context.Background()); version != "v2.0.0" {
		t.Fatalf("cached version = %q, want v2.0.0", version)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestNormalizeAgentVersionRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "short", "not-a-commit-version"} {
		if version := normalizeAgentVersion(value); version != "" {
			t.Fatalf("normalizeAgentVersion(%q) = %q, want empty", value, version)
		}
	}
}

func TestNormalizeAgentVersionAcceptsSemanticAndLegacyVersions(t *testing.T) {
	for input, expected := range map[string]string{" V2.0.0\n": "v2.0.0", "ab154322057948b9d9194d3432936aed09dc1d6e": "ab1543220579"} {
		if version := normalizeAgentVersion(input); version != expected {
			t.Fatalf("normalizeAgentVersion(%q) = %q, want %q", input, version, expected)
		}
	}
}

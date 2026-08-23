package iplocation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolverReturnsChineseLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/114.114.114.114" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("lang") != "zh-CN" {
			t.Fatalf("lang = %q", r.URL.Query().Get("lang"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"country":"中国","country_code":"CN","region":"江苏省","city":"南京","connection":{"isp":"中国电信"}}`))
	}))
	defer server.Close()

	resolver := New(server.Client(), server.URL)
	fixedTime := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
	resolver.now = func() time.Time { return fixedTime }
	location, err := resolver.Lookup(context.Background(), "114.114.114.114")
	if err != nil {
		t.Fatal(err)
	}
	if location.Scope != "public" || location.Country != "中国" || location.Province != "江苏省" || location.City != "南京" || location.ISP != "中国电信" {
		t.Fatalf("unexpected location: %+v", location)
	}
	if !location.ResolvedAt.Equal(fixedTime) {
		t.Fatalf("resolved_at = %s", location.ResolvedAt)
	}
}

func TestResolverDoesNotSendNonPublicAddress(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	resolver := New(server.Client(), server.URL)

	for _, address := range []string{"10.0.0.8", "203.0.113.8", "2001:db8::8"} {
		location, err := resolver.Lookup(context.Background(), address)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", address, err)
		}
		if location.Scope != "private" || location.Country != "内网/保留地址" {
			t.Fatalf("Lookup(%q) = %+v", address, location)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("provider received %d requests", requests.Load())
	}
}

func TestResolverMapsProviderRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	_, err := New(server.Client(), server.URL).Lookup(context.Background(), "8.8.8.8")
	if !errors.Is(err, ErrProviderRateLimited) {
		t.Fatalf("error = %v", err)
	}
}

func TestResolverRejectsInvalidAddress(t *testing.T) {
	_, err := New(nil, "").Lookup(context.Background(), "example.com")
	if !errors.Is(err, ErrInvalidAddress) {
		t.Fatalf("error = %v", err)
	}
}

func TestCleanFieldDropsProviderPlaceholders(t *testing.T) {
	for _, value := range []string{"0", "-", "   "} {
		if got := cleanField(value); got != "" {
			t.Fatalf("cleanField(%q) = %q", value, got)
		}
	}
	if got := cleanField("  中国电信  "); !strings.EqualFold(got, "中国电信") {
		t.Fatalf("cleanField returned %q", got)
	}
	if got := cleanField(strings.Repeat("长", maxFieldRunes+20)); len([]rune(got)) != maxFieldRunes {
		t.Fatalf("cleanField kept %d runes", len([]rune(got)))
	}
}

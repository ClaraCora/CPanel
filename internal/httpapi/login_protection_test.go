package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLoginProtectionLocksOnlyTheFailedSourceAndAccount(t *testing.T) {
	server := &Server{loginAttempts: make(map[string]loginAttempt)}
	now := time.Now()
	for range 3 {
		server.recordLoginFailure("admin", "198.51.100.8", "Admin@Example.com", 3, now)
	}
	if server.loginAllowed("admin", "198.51.100.8", "admin@example.com", 3, now) {
		t.Fatal("failed login pair was not locked")
	}
	if !server.loginAllowed("admin", "198.51.100.9", "admin@example.com", 3, now) {
		t.Fatal("different source IP was locked")
	}
	if !server.loginAllowed("admin", "198.51.100.8", "other@example.com", 3, now) {
		t.Fatal("different account was locked")
	}

	server.clearLoginFailures("admin", "198.51.100.8", "admin@example.com")
	if !server.loginAllowed("admin", "198.51.100.8", "admin@example.com", 3, now) {
		t.Fatal("successful login did not clear its matching rate limit")
	}
}

func TestLoginProtectionLocksAggregateSource(t *testing.T) {
	server := &Server{loginAttempts: make(map[string]loginAttempt)}
	now := time.Now()
	for index := range aggregateLoginFailureLimit(3) {
		server.recordLoginFailure("portal", "203.0.113.12", "user"+strconv.Itoa(index), 3, now)
	}
	if server.loginAllowed("portal", "203.0.113.12", "new-user", 3, now) {
		t.Fatal("aggregate source rate limit was not enforced")
	}
}

func TestLoginRateLimitedResponse(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/ca/ht/hh", nil)
	response := httptest.NewRecorder()
	writeLoginRateLimited(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	if response.Header().Get("Retry-After") != "900" {
		t.Fatalf("Retry-After = %q, want 900", response.Header().Get("Retry-After"))
	}
	if !strings.Contains(response.Body.String(), "LOGIN_RATE_LIMITED") {
		t.Fatalf("response does not contain generic rate-limit error: %s", response.Body.String())
	}
}

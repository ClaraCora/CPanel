package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cpanel/internal/domain"
)

func TestPortalProtectedRoutesRejectAnonymousRequests(t *testing.T) {
	server := &Server{loginAttempts: make(map[string]loginAttempt)}
	for _, test := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "current session", method: http.MethodGet, path: "/ca/edu/hh"},
		{name: "dashboard", method: http.MethodGet, path: "/ca/edu/zl"},
		{name: "password", method: http.MethodPatch, path: "/ca/edu/mm"},
		{name: "subscription rotation", method: http.MethodPost, path: "/ca/edu/dy"},
		{name: "logout", method: http.MethodPost, path: "/ca/edu/tc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s returned %d, want %d", test.method, test.path, response.Code, http.StatusUnauthorized)
			}
			if !strings.Contains(response.Body.String(), "PORTAL_AUTH_REQUIRED") {
				t.Fatalf("response does not identify missing portal auth: %s", response.Body.String())
			}
		})
	}
}

func TestRejectPortalAccountMutationForDelegatedSession(t *testing.T) {
	request := httptest.NewRequest(http.MethodPatch, "/ca/edu/mm", nil)
	response := httptest.NewRecorder()
	if !rejectPortalAccountMutation(response, request, domain.PortalSession{ReadOnly: true}, "只读") {
		t.Fatal("read-only delegated session was allowed to mutate the account")
	}
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "PORTAL_READ_ONLY") {
		t.Fatalf("read-only response = %d %s", response.Code, response.Body.String())
	}
}

func TestPortalGrantUsesShortLifetime(t *testing.T) {
	if portalGrantTTL.Seconds() != 90 {
		t.Fatalf("portal grant TTL = %s, want 90 seconds", portalGrantTTL)
	}
}

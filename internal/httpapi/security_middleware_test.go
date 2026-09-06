package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSensitiveResponsesDisableCaching(t *testing.T) {
	server := &Server{}
	handler := server.commonMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct {
		path    string
		noStore bool
	}{
		{path: "/ca/ht/zl", noStore: true},
		{path: "/ca/edu/zl", noStore: true},
		{path: "/ca/x/example", noStore: true},
		{path: "/ca/cc/fwq/jd", noStore: true},
		{path: "/ca/wj/corade-linux-amd64", noStore: false},
		{path: "/assets/index.js", noStore: false},
	} {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			got := response.Header().Get("Cache-Control")
			if test.noStore && got != "private, no-store" {
				t.Fatalf("Cache-Control = %q, want private, no-store", got)
			}
			if !test.noStore && got != "" {
				t.Fatalf("Cache-Control = %q, want empty", got)
			}
		})
	}
}

func TestCommonMiddlewareSetsBrowserSecurityHeaders(t *testing.T) {
	server := &Server{}
	handler := server.commonMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	for _, header := range []string{"Content-Security-Policy", "Cross-Origin-Opener-Policy", "Cross-Origin-Resource-Policy"} {
		if response.Header().Get(header) == "" {
			t.Fatalf("%s header is missing", header)
		}
	}
	if got := response.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS on HTTP request = %q, want empty", got)
	}

	secureRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secureRequest.Header.Set("X-Forwarded-Proto", "https")
	secureResponse := httptest.NewRecorder()
	handler.ServeHTTP(secureResponse, secureRequest)
	if got := secureResponse.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatal("HSTS missing for HTTPS proxy request")
	}
}

func TestClientIPUsesTrustedResolutionFromContext(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/ca/ht/zl", nil)
	request.RemoteAddr = "127.0.0.1:8256"
	request = request.WithContext(context.WithValue(request.Context(), clientIPKey, "198.51.100.42"))
	if got := clientIP(request); got != "198.51.100.42" {
		t.Fatalf("clientIP() = %q, want trusted client address", got)
	}
}

func TestLoginFailureSettingRequiresSafeRange(t *testing.T) {
	server := &Server{}
	router := chi.NewRouter()
	router.Patch("/ca/ht/sz/{section}", server.handleUpdateSettings)
	request := httptest.NewRequest(http.MethodPatch, "/ca/ht/sz/aq", strings.NewReader(`{"values":{"max_login_failures":2}}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnprocessableEntity)
	}
}

func TestUserAccessIPRecordsRequireAdministratorSession(t *testing.T) {
	server := &Server{}
	handler := server.commonMiddleware(server.requireAdmin(http.HandlerFunc(server.handleListUserAccessIPs)))
	request := httptest.NewRequest(http.MethodGet, "/ca/ht/yh/fwjl", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", got)
	}
}

func TestUserAccessIPLocationRequiresAdministratorSession(t *testing.T) {
	server := &Server{}
	handler := server.commonMiddleware(server.requireAdmin(http.HandlerFunc(server.handleResolveUserAccessIPLocation)))
	request := httptest.NewRequest(http.MethodPost, "/ca/ht/yh/fwjl/gs", strings.NewReader(`{"ip_address":"8.8.8.8"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", got)
	}
}

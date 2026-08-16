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

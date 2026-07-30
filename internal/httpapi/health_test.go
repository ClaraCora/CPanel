package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalHealthAllowsOnlyLoopbackHost(t *testing.T) {
	handler := localHealth(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	local := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8256/ca/jk/ch", nil)
	local.RemoteAddr = "127.0.0.1:42310"
	localResponse := httptest.NewRecorder()
	handler(localResponse, local)
	if localResponse.Code != http.StatusNoContent {
		t.Fatalf("local status = %d, want 204", localResponse.Code)
	}

	proxied := httptest.NewRequest(http.MethodGet, "https://cp.example.com/ca/jk/ch", nil)
	proxied.RemoteAddr = "127.0.0.1:42311"
	proxiedResponse := httptest.NewRecorder()
	handler(proxiedResponse, proxied)
	if proxiedResponse.Code != http.StatusNotFound {
		t.Fatalf("proxied status = %d, want 404", proxiedResponse.Code)
	}

	remote := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8256/ca/jk/ch", nil)
	remote.RemoteAddr = "203.0.113.8:42312"
	remoteResponse := httptest.NewRecorder()
	handler(remoteResponse, remote)
	if remoteResponse.Code != http.StatusNotFound {
		t.Fatalf("remote status = %d, want 404", remoteResponse.Code)
	}
}

func TestLegacyPublicPathsReturnNotFound(t *testing.T) {
	handler := (&Server{}).Handler()
	for _, path := range []string{"/api/ops/v1/session", "/health/live", "/health/ready"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, response.Code)
		}
	}
}

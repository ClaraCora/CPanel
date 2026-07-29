package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIndexForApplicationRoutes(t *testing.T) {
	for _, route := range []string{"/", "/nodes/example/edit", "/settings"} {
		request := httptest.NewRequest(http.MethodGet, route, nil)
		response := httptest.NewRecorder()

		Handler().ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d", route, response.Code)
		}
		if !strings.Contains(response.Body.String(), `<div id="root"></div>`) {
			t.Fatalf("GET %s did not return the application shell", route)
		}
	}
}

func TestHandlerDoesNotMaskBackendOrMissingAssetRoutes(t *testing.T) {
	for _, route := range []string{"/api/unknown", "/ca/unknown", "/health/unknown", "/assets/missing.js"} {
		request := httptest.NewRequest(http.MethodGet, route, nil)
		response := httptest.NewRecorder()

		Handler().ServeHTTP(response, request)

		if response.Code != http.StatusNotFound {
			t.Fatalf("GET %s returned %d, want 404", route, response.Code)
		}
	}
}

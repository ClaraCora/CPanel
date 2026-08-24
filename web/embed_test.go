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

func TestPublicEntryDoesNotContainAdministrativeContent(t *testing.T) {
	entries, err := assets.ReadDir("dist/assets")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"订阅", "节点", "流量", "服务器", "机场",
		"vless://", "ss://", "cps_demo", "subscription_url", "traffic_used_bytes",
	}
	publicScripts := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		publicScripts++
		body, readErr := assets.ReadFile("dist/assets/" + entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, value := range forbidden {
			if strings.Contains(string(body), value) {
				t.Fatalf("public script %s contains administrative content %q", entry.Name(), value)
			}
		}
	}
	if publicScripts == 0 {
		t.Fatal("no public entry script found")
	}
}

func TestProtectedAssetsUsePrivateCache(t *testing.T) {
	entries, err := assets.ReadDir("dist/assets/secure")
	if err != nil {
		t.Fatal(err)
	}
	var assetName string
	for _, entry := range entries {
		if !entry.IsDir() {
			assetName = entry.Name()
			break
		}
	}
	if assetName == "" {
		t.Fatal("no protected asset found")
	}

	request := httptest.NewRequest(http.MethodGet, "/assets/secure/"+assetName, nil)
	response := httptest.NewRecorder()
	Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("protected asset status = %d, want 200", response.Code)
	}
	if !strings.HasPrefix(response.Header().Get("Cache-Control"), "private,") {
		t.Fatalf("protected asset cache control = %q, want private", response.Header().Get("Cache-Control"))
	}
	if !strings.Contains(response.Header().Get("Vary"), "Cookie") {
		t.Fatalf("protected asset vary = %q, want Cookie", response.Header().Get("Vary"))
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

func TestHandlerRejectsParentPathSegments(t *testing.T) {
	entries, err := assets.ReadDir("dist/assets/secure")
	if err != nil {
		t.Fatal(err)
	}
	var assetName string
	for _, entry := range entries {
		if !entry.IsDir() {
			assetName = entry.Name()
			break
		}
	}
	if assetName == "" {
		t.Fatal("no protected asset found")
	}

	for _, route := range []string{
		"/assets/portal/../secure/" + assetName,
		"/assets/%2e%2e/secure/" + assetName,
		"/foo/../assets/secure/" + assetName,
	} {
		request := httptest.NewRequest(http.MethodGet, route, nil)
		response := httptest.NewRecorder()
		Handler().ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET %s returned %d, want 404", route, response.Code)
		}
	}
}

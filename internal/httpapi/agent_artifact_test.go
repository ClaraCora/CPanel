package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"cpanel/internal/config"
	"github.com/go-chi/chi/v5"
)

func TestAgentArtifactServesAllowedBinary(t *testing.T) {
	directory := t.TempDir()
	want := []byte("test agent binary")
	if err := os.WriteFile(filepath.Join(directory, "corade-linux-amd64"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	server := &Server{cfg: config.Config{AgentArtifactDir: directory}}
	router := chi.NewRouter()
	router.Get("/ca/wj/{artifact}", server.handleAgentArtifact)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ca/wj/corade-linux-amd64", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if string(response.Body.Bytes()) != string(want) {
		t.Fatalf("body = %q, want %q", response.Body.Bytes(), want)
	}
	if got := response.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestAgentArtifactRejectsUnknownName(t *testing.T) {
	server := &Server{cfg: config.Config{AgentArtifactDir: t.TempDir()}}
	router := chi.NewRouter()
	router.Get("/ca/wj/{artifact}", server.handleAgentArtifact)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ca/wj/other", nil))

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestAgentArtifactServesChecksum(t *testing.T) {
	directory := t.TempDir()
	want := []byte("0123456789  corade-linux-amd64\n")
	if err := os.WriteFile(filepath.Join(directory, "corade-linux-amd64.sha256"), want, 0o600); err != nil {
		t.Fatal(err)
	}
	server := &Server{cfg: config.Config{AgentArtifactDir: directory}}
	router := chi.NewRouter()
	router.Get("/ca/wj/{artifact}", server.handleAgentArtifact)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ca/wj/corade-linux-amd64.sha256", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if string(response.Body.Bytes()) != string(want) {
		t.Fatalf("body = %q, want %q", response.Body.Bytes(), want)
	}
}

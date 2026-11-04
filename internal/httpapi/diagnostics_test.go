package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestPanicDiagnosticsKeepRequestIDAndAvoidSecretURLs(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	s := &Server{}
	router := chi.NewRouter()
	router.Use(recoveryMiddleware)
	router.Use(s.commonMiddleware)
	router.Get("/ca/x/{token}", func(http.ResponseWriter, *http.Request) { panic("test failure") })
	request := httptest.NewRequest(http.MethodGet, "/ca/x/secret-subscription-token?grant=secret-grant", nil)
	request.Header.Set("X-Request-ID", "req_diagnostics")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", response.Code)
	}
	var body struct {
		Meta struct {
			RequestID string `json:"request_id"`
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.RequestID != "req_diagnostics" {
		t.Fatalf("response request id=%q", body.Meta.RequestID)
	}
	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["request_id"] != "req_diagnostics" || entry["route"] != "/ca/x/{token}" || entry["method"] != "GET" || entry["stack"] == "" {
		t.Fatalf("missing diagnostics: %+v", entry)
	}
	for _, secret := range []string{"secret-subscription-token", "secret-grant"} {
		if strings.Contains(logs.String(), secret) || strings.Contains(response.Body.String(), secret) {
			t.Fatalf("diagnostics exposed %s", secret)
		}
	}
}

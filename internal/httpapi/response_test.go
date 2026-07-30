package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cpanel/internal/store"
)

func TestWriteStoreErrorIdentifiesNodePortConflict(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/ops/v1/nodes", nil)
	response := httptest.NewRecorder()

	writeStoreError(response, request, store.ErrNodePortInUse)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	var body envelope
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error == nil || body.Error.Code != "NODE_PORT_IN_USE" {
		t.Fatalf("error = %#v, want NODE_PORT_IN_USE", body.Error)
	}
	if body.Error.Fields["server_port"] != "该端口已被占用" {
		t.Fatalf("server_port field = %q, want occupied message", body.Error.Fields["server_port"])
	}
}

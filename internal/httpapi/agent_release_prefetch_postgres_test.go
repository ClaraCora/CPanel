package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cpanel/internal/domain"
)

func TestSlowAgentReleaseLookupHoldsNoDatabaseLocksPostgres(t *testing.T) {
	for _, test := range []struct {
		name, method, path string
		write              func(context.Context, *Server, http.Handler) error
	}{
		{name: "upgrade does not block unrelated administrative write", method: "POST", path: "/ca/ht/fwq/mch_release/sj",
			write: func(ctx context.Context, _ *Server, handler http.Handler) error {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, consistencyRequest("POST", "/ca/ht/fwq", `{"name":"Independent create"}`, "independent-release-create", "").WithContext(ctx))
				if response.Code != http.StatusCreated {
					return fmt.Errorf("independent create: %d %s", response.Code, response.Body.String())
				}
				return nil
			}},
		{name: "list does not hold presence maintenance row lock", method: "GET", path: "/ca/ht/fwq",
			write: func(ctx context.Context, server *Server, _ http.Handler) error {
				// A real heartbeat needs the same machine row that presence
				// maintenance previously kept locked during the release lookup.
				return server.store.RecordMachineHeartbeat(ctx, domain.AgentMachine{ID: "mch_release"}, "v2.0.8", "xray", nil, nil)
			}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, pool, handler := consistencyPostgres(t)
			heartbeatAge := "0 seconds"
			if test.method == "GET" {
				heartbeatAge = "10 minutes"
			}
			if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name,status,agent_version,last_heartbeat_at)
				VALUES('mch_release','Release machine','online','v2.0.8',now()-$1::interval)`, heartbeatAge); err != nil {
				t.Fatal(err)
			}
			started, release := make(chan struct{}), make(chan struct{})
			var startedOnce, releaseOnce sync.Once
			var calls atomic.Int32
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				startedOnce.Do(func() { close(started) })
				select {
				case <-release:
					fmt.Fprint(w, "v2.0.9")
				case <-r.Context().Done():
				}
			}))
			t.Cleanup(remote.Close)
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			server.agentRelease = newAgentReleaseResolver(remote.Client(), remote.URL)
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() { done <- consistencyCall(handler, test.method, test.path, "", "", "") }()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("release lookup did not start")
			}
			writeCtx, cancel := context.WithTimeout(t.Context(), time.Second)
			err := test.write(writeCtx, server, handler)
			cancel()
			if err != nil {
				t.Errorf("external lookup blocked an independent database operation: %v", err)
			}
			unblock()
			select {
			case response := <-done:
				want := http.StatusOK
				if test.method == "POST" {
					want = http.StatusAccepted
				}
				if response.Code != want {
					t.Fatalf("original request: %d %s", response.Code, response.Body.String())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("original request did not finish after release became available")
			}
			if calls.Load() != 1 {
				t.Fatalf("release lookup called %d times", calls.Load())
			}
		})
	}
}

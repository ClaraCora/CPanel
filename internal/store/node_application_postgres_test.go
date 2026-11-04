package store

import (
	"encoding/json"
	"testing"

	"cpanel/internal/domain"
)

func TestNodeApplicationRequiresExplicitSuccessPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	for _, query := range []string{
		`INSERT INTO machines(id,name) VALUES('mch_apply','Application machine')`,
		`INSERT INTO nodes(id,machine_id,name,protocol,server_port,status,current_revision) VALUES('nod_apply','mch_apply','Application node','vless',443,'published',3)`,
	} {
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	s := New(pool)
	var nodeAgentID int64
	if err := pool.QueryRow(ctx, `SELECT agent_id FROM nodes WHERE id='nod_apply'`).Scan(&nodeAgentID); err != nil {
		t.Fatal(err)
	}
	send := func(kind string, revision int, status, stage, message string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"events": []any{map[string]any{
			"type": kind, "node_id": nodeAgentID,
			"data": map[string]any{"revision": revision, "status": status, "stage": stage, "error": message},
		}}})
		if _, err := s.RecordTelemetryBatch(ctx, "mch_apply", domain.MustID("application"), body); err != nil {
			t.Fatal(err)
		}
	}
	check := func(applied, received int, status, message string) {
		t.Helper()
		node, err := s.GetNode(ctx, "nod_apply")
		if err != nil {
			t.Fatal(err)
		}
		if node.AppliedRevision != applied || node.ReceivedRevision != received || node.ApplyStatus != status || node.LastError != message {
			t.Fatalf("unexpected application state: %+v", node)
		}
	}
	send("node.telemetry", 3, "", "", "")
	check(0, 0, "", "")
	send("node.application", 1, "received", "fetch", "")
	check(0, 1, "received", "")
	send("node.application", 1, "applied", "kernel", "")
	check(1, 1, "applied", "")
	send("node.application", 2, "received", "fetch", "")
	send("node.application", 2, "failed", "validate", "unsupported protocol")
	check(1, 2, "failed", "unsupported protocol")
	// A delayed receive or old success must not erase the newer failure.
	send("node.application", 2, "received", "fetch", "")
	send("node.application", 1, "applied", "kernel", "")
	check(1, 2, "failed", "unsupported protocol")
	send("node.application", 2, "applied", "kernel", "")
	check(2, 2, "applied", "")
	// A future version or another machine cannot confirm this node.
	send("node.application", 4, "applied", "kernel", "")
	check(2, 2, "applied", "")
	if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name) VALUES('mch_other','Other machine')`); err != nil {
		t.Fatal(err)
	}
	wrongMachine, _ := json.Marshal(map[string]any{"events": []any{map[string]any{
		"type": "node.application", "node_id": nodeAgentID, "data": map[string]any{"revision": 3, "status": "applied", "stage": "kernel"},
	}}})
	if _, err := s.RecordTelemetryBatch(ctx, "mch_other", domain.MustID("application"), wrongMachine); err != nil {
		t.Fatal(err)
	}
	check(2, 2, "applied", "")
	send("node.application", 3, "received", "fetch", "")
	send("node.application", 3, "failed", "kernel", "bind failed")
	check(2, 3, "failed", "bind failed")
	send("node.application", 3, "applied", "kernel", "")
	check(3, 3, "applied", "")
	send("node.application", 3, "failed", "kernel", "late failure")
	check(3, 3, "applied", "")
}

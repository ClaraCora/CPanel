package store

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeOptionalID(t *testing.T) {
	empty := "  "
	if value := normalizeOptionalID(&empty); value != nil {
		t.Fatalf("normalizeOptionalID(empty) = %q, want nil", *value)
	}

	id := "  rte_example  "
	value := normalizeOptionalID(&id)
	if value == nil || *value != "rte_example" {
		t.Fatalf("normalizeOptionalID(id) = %v, want rte_example", value)
	}
}

func TestMapNodeErrorIdentifiesPortConflict(t *testing.T) {
	for _, constraint := range []string{"nodes_machine_id_server_port_key", "nodes_machine_port_active_unique"} {
		err := mapNodeError(&pgconn.PgError{Code: "23505", ConstraintName: constraint})
		if !errors.Is(err, ErrNodePortInUse) {
			t.Fatalf("constraint %q mapped to %v, want ErrNodePortInUse", constraint, err)
		}
	}
}

func TestMachineNodesReplacePayload(t *testing.T) {
	var payload struct {
		NodeID int64 `json:"node_id"`
	}
	if err := json.Unmarshal(machineNodesReplacePayload(11), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.NodeID != 11 {
		t.Fatalf("node_id = %d, want 11", payload.NodeID)
	}
}

func TestEffectiveMachineStatusUsesHeartbeatThreshold(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-179 * time.Second)
	stale := now.Add(-181 * time.Second)

	tests := []struct {
		name          string
		status        string
		lastHeartbeat *time.Time
		want          string
	}{
		{name: "recent heartbeat", status: "online", lastHeartbeat: &recent, want: "online"},
		{name: "stale heartbeat", status: "online", lastHeartbeat: &stale, want: "offline"},
		{name: "missing heartbeat", status: "online", want: "offline"},
		{name: "disabled is preserved", status: "disabled", lastHeartbeat: &stale, want: "disabled"},
		{name: "pending is preserved", status: "pending", want: "pending"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := effectiveMachineStatus(test.status, test.lastHeartbeat, now, 180*time.Second); got != test.want {
				t.Fatalf("effectiveMachineStatus() = %q, want %q", got, test.want)
			}
		})
	}
}

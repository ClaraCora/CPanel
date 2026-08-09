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

func TestRankingDateOffsetsForMonth(t *testing.T) {
	tests := []struct {
		name       string
		currentDay time.Time
		wantStart  int
		wantEnd    int
	}{
		{name: "first day", currentDay: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC), wantStart: 0, wantEnd: 1},
		{name: "month to date", currentDay: time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC), wantStart: -8, wantEnd: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end := rankingDateOffsets("month", test.currentDay)
			if start != test.wantStart || end != test.wantEnd {
				t.Fatalf("rankingDateOffsets(month) = (%d, %d), want (%d, %d)", start, end, test.wantStart, test.wantEnd)
			}
		})
	}
}

func TestTrafficMonthBoundsUsesSiteTimezone(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	now := time.Date(2026, 7, 31, 16, 30, 0, 0, time.UTC)
	start, end, nextReset := trafficMonthBounds(now, location)
	if start != "2026-08-01" || end != "2026-09-01" {
		t.Fatalf("trafficMonthBounds() = (%q, %q), want (2026-08-01, 2026-09-01)", start, end)
	}
	wantReset := time.Date(2026, 9, 1, 0, 0, 0, 0, location)
	if !nextReset.Equal(wantReset) {
		t.Fatalf("next reset = %s, want %s", nextReset, wantReset)
	}
}

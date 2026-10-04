package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDecodeTelemetryBatch(t *testing.T) {
	payload := json.RawMessage(`{"events":[{"type":"node.telemetry","node_id":12,"occurred_at":"2026-07-29T12:00:00Z","data":{"revision":7,"traffic":{"9":[100,200]}}}]}`)
	batch, err := decodeTelemetryBatch(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Events) != 1 || batch.Events[0].Type != "node.telemetry" || batch.Events[0].NodeID != 12 {
		t.Fatalf("unexpected batch: %+v", batch)
	}
	var data nodeTelemetryData
	if err := json.Unmarshal(batch.Events[0].Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.Revision != 7 || data.Traffic["9"] != [2]int64{100, 200} {
		t.Fatalf("unexpected telemetry data: %+v", data)
	}
}

func TestDecodeTelemetryBatchRejectsMissingEvents(t *testing.T) {
	if _, err := decodeTelemetryBatch(json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected missing events error")
	}
}

func TestTelemetryTimeUsesUTCAndFallsBack(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	got := boundedTelemetryTime("2026-07-29T20:00:00+08:00", now)
	want := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("telemetryTime = %s, want %s", got, want)
	}
	before := time.Now().UTC().Add(-time.Second)
	fallback := telemetryTime("invalid")
	if fallback.Before(before) || fallback.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("unexpected fallback time %s", fallback)
	}
}

func TestBoundedTelemetryTimeRejectsClockSkew(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	if got := boundedTelemetryTime("2026-08-24T11:59:00Z", now); !got.Equal(now.Add(-time.Minute)) {
		t.Fatalf("valid telemetry time = %s", got)
	}
	if got := boundedTelemetryTime("2026-08-25T12:00:00Z", now); !got.Equal(now) {
		t.Fatalf("future telemetry time = %s, want receive time %s", got, now)
	}
	if got := boundedTelemetryTime("2025-01-01T00:00:00Z", now); !got.Equal(now) {
		t.Fatalf("stale telemetry time = %s, want receive time %s", got, now)
	}
}

func TestTelemetryDayUsesSiteTimezoneAcrossUTCMidnight(t *testing.T) {
	sampledAt := boundedTelemetryTime("2026-07-30T16:22:18Z", time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC))
	if got, want := telemetryDay(sampledAt, loadTrafficLocation("Asia/Shanghai")), "2026-07-31"; got != want {
		t.Fatalf("telemetryDay = %s, want %s", got, want)
	}
}

func TestLoadTrafficLocationFallsBackToAsiaShanghai(t *testing.T) {
	sampledAt := boundedTelemetryTime("2026-07-30T16:22:18Z", time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC))
	if got, want := telemetryDay(sampledAt, loadTrafficLocation("not/a-timezone")), "2026-07-31"; got != want {
		t.Fatalf("fallback telemetryDay = %s, want %s", got, want)
	}
}

func TestAgentUpgradeTransitions(t *testing.T) {
	tests := []struct {
		current string
		next    string
		want    bool
	}{
		{current: "dispatched", next: "acknowledged", want: true},
		{current: "dispatched", next: "failed", want: true},
		{current: "dispatched", next: "succeeded", want: true},
		{current: "acknowledged", next: "succeeded", want: true},
		{current: "acknowledged", next: "failed", want: true},
		{current: "acknowledged", next: "timed_out", want: true},
		{current: "timed_out", next: "succeeded", want: true},
		{current: "timed_out", next: "failed", want: true},
		{current: "queued", next: "acknowledged", want: false},
		{current: "succeeded", next: "failed", want: false},
	}
	for _, test := range tests {
		if got := validAgentUpgradeTransition(test.current, test.next); got != test.want {
			t.Errorf("validAgentUpgradeTransition(%q, %q) = %t, want %t", test.current, test.next, got, test.want)
		}
	}
}

func TestAgentVersionComparison(t *testing.T) {
	for _, test := range []struct {
		current string
		target  string
		want    bool
	}{
		{current: "v2.0.3+abc123", target: "v2.0.3", want: true},
		{current: "corade-v2.0.3", target: "v2.0.3", want: true},
		{current: "v2.0.2", target: "v2.0.3", want: false},
		{current: "v2.0.3+abc123", target: "latest", want: true},
		{current: "", target: "latest", want: false},
	} {
		if got := agentTargetVersionMatches(test.current, test.target); got != test.want {
			t.Errorf("agentTargetVersionMatches(%q, %q) = %t, want %t", test.current, test.target, got, test.want)
		}
	}
}

func TestBoundedTelemetryTimeRejectsExtremeClockSkew(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	for _, value := range []string{"2026-07-31T12:01:00Z", "2026-07-28T11:59:59Z", "not-a-time"} {
		if got := boundedTelemetryTime(value, now); !got.Equal(now) {
			t.Fatalf("boundedTelemetryTime(%q) = %s, want %s", value, got, now)
		}
	}
}

func TestBoundedTelemetryTimePreservesReasonableTimestamp(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	want := now.Add(-2 * time.Hour)
	if got := boundedTelemetryTime(want.Format(time.RFC3339Nano), now); !got.Equal(want) {
		t.Fatalf("boundedTelemetryTime() = %s, want %s", got, want)
	}
}

func TestNormalizeDeviceReportsCanonicalizesAndSorts(t *testing.T) {
	reports := normalizeDeviceReports(map[string][]string{
		"12": {" 2001:0db8::1 ", "2001:db8:0:0::1", "invalid"},
		"3":  {"::ffff:192.0.2.4", "192.0.2.4", "198.51.100.9"},
		"0":  {"203.0.113.8"},
		"x":  {"203.0.113.9"},
	})
	if len(reports) != 2 {
		t.Fatalf("reports length = %d, want 2", len(reports))
	}
	if reports[0].AgentID != 3 || reports[1].AgentID != 12 {
		t.Fatalf("reports are not sorted by user: %+v", reports)
	}
	if got, want := reports[0].Addresses, []string{"192.0.2.4", "198.51.100.9"}; !equalStrings(got, want) {
		t.Fatalf("IPv4 addresses = %v, want %v", got, want)
	}
	if got, want := reports[1].Addresses, []string{"2001:db8::1"}; !equalStrings(got, want) {
		t.Fatalf("IPv6 addresses = %v, want %v", got, want)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

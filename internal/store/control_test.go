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
	got := telemetryTime("2026-07-29T20:00:00+08:00")
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

func TestTelemetryDayUsesSiteTimezoneAcrossUTCMidnight(t *testing.T) {
	sampledAt := telemetryTime("2026-07-30T16:22:18Z")
	if got, want := telemetryDay(sampledAt, loadTrafficLocation("Asia/Shanghai")), "2026-07-31"; got != want {
		t.Fatalf("telemetryDay = %s, want %s", got, want)
	}
}

func TestLoadTrafficLocationFallsBackToAsiaShanghai(t *testing.T) {
	sampledAt := telemetryTime("2026-07-30T16:22:18Z")
	if got, want := telemetryDay(sampledAt, loadTrafficLocation("not/a-timezone")), "2026-07-31"; got != want {
		t.Fatalf("fallback telemetryDay = %s, want %s", got, want)
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

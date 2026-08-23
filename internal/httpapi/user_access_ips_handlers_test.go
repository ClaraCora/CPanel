package httpapi

import (
	"testing"
	"time"
)

func TestUserAccessIPLocationLookupCooldown(t *testing.T) {
	server := &Server{}
	startedAt := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
	if !server.allowUserAccessIPLocationLookup("8.8.8.8", startedAt) {
		t.Fatal("first lookup was rejected")
	}
	if server.allowUserAccessIPLocationLookup("8.8.8.8", startedAt.Add(30*time.Second)) {
		t.Fatal("lookup inside cooldown was allowed")
	}
	if !server.allowUserAccessIPLocationLookup("8.8.8.8", startedAt.Add(time.Minute)) {
		t.Fatal("lookup after cooldown was rejected")
	}
}

func TestUserAccessIPLocationLookupPrunesOldAddresses(t *testing.T) {
	server := &Server{}
	startedAt := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
	server.allowUserAccessIPLocationLookup("8.8.8.8", startedAt)
	server.allowUserAccessIPLocationLookup("1.1.1.1", startedAt.Add(userAccessIPLookupRetention+time.Second))
	if _, exists := server.ipLocationLookups["8.8.8.8"]; exists {
		t.Fatal("expired lookup was not pruned")
	}
}

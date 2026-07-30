package httpapi

import "testing"

func TestCanonicalUserUUID(t *testing.T) {
	got, valid := canonicalUserUUID("550E8400-E29B-41D4-A716-446655440000")
	if !valid || got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("canonicalUserUUID() = %q, %v", got, valid)
	}
}

func TestCanonicalUserUUIDRejectsInvalidValue(t *testing.T) {
	if _, valid := canonicalUserUUID("not-a-uuid"); valid {
		t.Fatal("canonicalUserUUID() accepted an invalid value")
	}
}

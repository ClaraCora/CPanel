package main

import "testing"

func TestParseIDSet(t *testing.T) {
	got, err := parseIDSet("5, 70")
	if err != nil {
		t.Fatal(err)
	}
	if !got[5] || !got[70] || len(got) != 2 {
		t.Fatalf("parseIDSet() = %v", got)
	}
}

func TestParseIDSetRejectsInvalidID(t *testing.T) {
	if _, err := parseIDSet("5,nope"); err == nil {
		t.Fatal("parseIDSet() accepted an invalid ID")
	}
}

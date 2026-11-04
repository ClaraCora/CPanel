package store

import (
	"testing"
	"time"
)

func TestDefaultExpiryUsesCalendarDaysInSiteTimezone(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 7, 12, 30, 0, 0, location)
	expires, err := userExpiry("default", nil, 2, now, location)
	if err != nil || expires == nil || expires.In(location).Format("2006-01-02 15:04") != "2026-03-09 12:30" {
		t.Fatalf("default expiry=%v err=%v", expires, err)
	}
	if expires.Sub(now) != 47*time.Hour {
		t.Fatalf("DST calendar expiry duration=%v, want 47h", expires.Sub(now))
	}
	if expires, err := userExpiry("default", nil, 0, now, location); err != nil || expires != nil {
		t.Fatalf("zero default days expiry=%v err=%v", expires, err)
	}
}

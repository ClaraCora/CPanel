package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoginAdmissionBoundsConcurrentHashWork(t *testing.T) {
	s := &Server{}
	now := time.Now()
	var group sync.WaitGroup
	var releasesMu sync.Mutex
	var releases []func()
	for i := range 64 {
		group.Go(func() {
			release, ok := s.beginLogin("admin", fmt.Sprintf("192.0.2.%d", i), fmt.Sprintf("user%d", i), 8, now)
			if ok {
				releasesMu.Lock()
				releases = append(releases, release)
				releasesMu.Unlock()
			}
		})
	}
	group.Wait()
	if len(releases) != maxConcurrentPasswordChecks {
		t.Fatalf("admitted %d of 64, want %d", len(releases), maxConcurrentPasswordChecks)
	}
	for _, release := range releases {
		release()
		release()
	}
	if s.passwordChecks != 0 {
		t.Fatal("capacity was not returned exactly once")
	}
	if release, ok := s.beginLogin("portal", "198.51.100.2", "new", 8, now); !ok {
		t.Fatal("released capacity not reusable")
	} else {
		release()
	}
}

func TestLoginAdmissionAccountAndSharedSourceLimits(t *testing.T) {
	s := &Server{}
	now := time.Now()
	release, ok := s.beginLogin("admin", "192.0.2.1", "USER@example.com", 8, now)
	if !ok {
		t.Fatal("first login rejected")
	}
	if _, ok := s.beginLogin("admin", "192.0.2.2", " user@example.com ", 8, now); ok {
		t.Fatal("account parallelism allowed across IPs")
	}
	release()
	for i := range maxLoginStartsPerIP {
		release, ok = s.beginLogin("portal", "198.51.100.3", fmt.Sprintf("user%d", i), 8, now)
		if !ok {
			t.Fatalf("request %d rejected early", i)
		}
		release()
	}
	if _, ok := s.beginLogin("admin", "198.51.100.3", "another", 8, now); ok {
		t.Fatal("admin bypassed shared IP rate budget")
	}
	release, ok = s.beginLogin("admin", "198.51.100.3", "another", 8, now.Add(loginAdmissionWindow))
	if !ok {
		t.Fatal("rate window did not refill")
	}
	release()
}

func TestBothLoginEndpointsRejectBeforePasswordLookupWhenFull(t *testing.T) {
	for _, portal := range []bool{false, true} {
		s := &Server{passwordChecks: maxConcurrentPasswordChecks}
		input := `{"email":"admin@example.com","password":"valid-password"}`
		handler := s.handleLogin
		if portal {
			input = `{"login":"user@example.com","password":"valid-password"}`
			handler = s.handlePortalLogin
		}
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(input))
		w := httptest.NewRecorder()
		handler(w, r) // A store access would panic: no database is attached.
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("portal=%v status=%d", portal, w.Code)
		}
	}
}

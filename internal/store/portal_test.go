package store

import (
	"strings"
	"testing"
)

func TestPortalEligibleUsersCTEEnforcesRuntimeEntitlement(t *testing.T) {
	for _, predicate := range []string{
		"g.status='active'",
		"u.status='active'",
		"u.expires_at IS NULL OR u.expires_at > now()",
	} {
		if !strings.Contains(portalEligibleUsersCTE, predicate) {
			t.Fatalf("portal entitlement query is missing %q", predicate)
		}
	}
}

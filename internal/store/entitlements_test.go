package store

import (
	"strings"
	"testing"
)

func TestRuntimeEligibleUsersCTEEnforcesAccessGroupAndUserState(t *testing.T) {
	for _, predicate := range []string{
		"g.status='active'",
		"u.status='active'",
		"u.expires_at IS NULL OR u.expires_at > now()",
	} {
		if !strings.Contains(runtimeEligibleUsersCTE, predicate) {
			t.Fatalf("runtime entitlement query is missing %q", predicate)
		}
	}
}

func TestRuntimeEligibleUsersCTEDoesNotRevokeExistingDisabledPlanAssignments(t *testing.T) {
	for _, forbidden := range []string{"p.status='active'", "p.status = 'active'"} {
		if strings.Contains(runtimeEligibleUsersCTE, forbidden) {
			t.Fatalf("runtime entitlement query unexpectedly filters disabled plans with %q", forbidden)
		}
	}
}

func TestSubscriptionEndpointAccessScopeFilter(t *testing.T) {
	for _, fragment := range []string{
		"e.access_scope",
		"u.role='admin' OR e.access_scope='default'",
		"NOT EXISTS (SELECT 1 FROM node_endpoints configured WHERE configured.node_id=n.id)",
	} {
		if !strings.Contains(subscriptionEndpointQuery, fragment) {
			t.Fatalf("subscription endpoint query is missing %q", fragment)
		}
	}
}

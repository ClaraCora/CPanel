package store

import (
	"errors"
	"testing"

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

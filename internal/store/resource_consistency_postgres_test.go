package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestValidateRoutePolicyMutationPostgres(t *testing.T) {
	databaseURL := os.Getenv("CPANEL_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set CPANEL_TEST_DATABASE_URL to run PostgreSQL regression tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())

	for _, test := range []struct {
		name, scope, status, defaultTag string
		slot, secondSlot, nodeStatus    string
		kernel                          string
		wantErr                         error
	}{
		{name: "unbound policy", scope: "default", status: "published"},
		{name: "default binding", scope: "default", status: "published", slot: "default"},
		{name: "admin binding", scope: "admin", status: "published", slot: "admin"},
		{name: "member binding", scope: "member", status: "published", slot: "member"},
		{name: "scope mismatch", scope: "member", status: "published", slot: "admin", wantErr: ErrRoutePolicyScopeMismatch},
		{name: "all binding slots are checked", scope: "default", status: "published", slot: "default", secondSlot: "admin", wantErr: ErrRoutePolicyScopeMismatch},
		{name: "bound policy cannot be disabled", scope: "default", status: "disabled", slot: "default", wantErr: ErrRoutePolicyInUse},
		{name: "archived binding is ignored", scope: "member", status: "disabled", slot: "admin", nodeStatus: "archived"},
		{name: "available outbound", scope: "admin", status: "published", slot: "admin", defaultTag: "warp"},
		{name: "missing outbound", scope: "admin", status: "published", slot: "admin", defaultTag: "missing", wantErr: ErrRouteOutboundUnavailable},
		{name: "outbound kernel mismatch", scope: "admin", status: "published", slot: "admin", defaultTag: "warp", kernel: "singbox", wantErr: ErrOutboundKernelUnsupported},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			// These transaction-local tables isolate every case from application
			// data, and exercise the real PostgreSQL parser and row-lock behavior.
			_, err = tx.Exec(ctx, `CREATE TEMP TABLE nodes (
				id TEXT PRIMARY KEY, status TEXT, kernel_type TEXT,
				route_policy_id TEXT, admin_route_policy_id TEXT, member_route_policy_id TEXT
			) ON COMMIT DROP;
			CREATE TEMP TABLE outbounds (
				tag TEXT, protocol TEXT, status TEXT, proxy_tag TEXT, kernel_support TEXT[]
			) ON COMMIT DROP;
			INSERT INTO pg_temp.outbounds VALUES ('warp','socks','active','',ARRAY['xray']);`)
			if err != nil {
				t.Fatal(err)
			}
			if test.slot != "" {
				status, kernel := test.nodeStatus, test.kernel
				if status == "" {
					status = "published"
				}
				if kernel == "" {
					kernel = "xray"
				}
				var defaultID, adminID, memberID *string
				policyID := "rte_test"
				for _, slot := range []string{test.slot, test.secondSlot} {
					switch slot {
					case "default":
						defaultID = &policyID
					case "admin":
						adminID = &policyID
					case "member":
						memberID = &policyID
					}
				}
				_, err = tx.Exec(ctx, `INSERT INTO pg_temp.nodes
					(id,status,kernel_type,route_policy_id,admin_route_policy_id,member_route_policy_id)
					VALUES ('nod_test',$1,$2,$3,$4,$5)`, status, kernel, defaultID, adminID, memberID)
				if err != nil {
					t.Fatal(err)
				}
			}
			err = (&Store{}).validateRoutePolicyMutationTx(ctx, tx, "rte_test", test.status, test.scope, test.defaultTag, nil)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("validateRoutePolicyMutationTx() = %v, want %v", err, test.wantErr)
			}
		})
	}
}

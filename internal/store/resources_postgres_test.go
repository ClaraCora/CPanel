package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"cpanel/internal/database"
	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func resourcePostgresPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("CPANEL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CPANEL_TEST_DATABASE_URL for the real PostgreSQL resource regressions")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := domain.MustID("resource_test")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return ctx, pool
}

func TestArchiveOutboundReferencesPostgres(t *testing.T) {
	const routeRule = `[{"action":{"type":"route","target":"TARGET"}}]`
	const disabledRule = `[{"disabled":true,"action":{"type":"route","target":"TARGET"}}]`
	for _, test := range []struct {
		name             string
		proxyReference   bool
		policyStatus     string
		defaultReference bool
		revisions        []string
		wantErr          error
	}{
		{name: "unreferenced with empty upstream"},
		{name: "chain reference", proxyReference: true, wantErr: ErrOutboundInUse},
		{name: "published default", policyStatus: "published", defaultReference: true, wantErr: ErrOutboundInUse},
		{name: "published rule", policyStatus: "published", revisions: []string{routeRule}, wantErr: ErrOutboundInUse},
		{name: "disabled rule", policyStatus: "published", revisions: []string{disabledRule}},
		{name: "archived policy", policyStatus: "archived", defaultReference: true, revisions: []string{routeRule}},
		{name: "disabled policy", policyStatus: "disabled", defaultReference: true, revisions: []string{routeRule}},
		{name: "old revision ignored", policyStatus: "published", revisions: []string{routeRule, `[]`}},
		{name: "latest revision referenced", policyStatus: "published", revisions: []string{`[]`, routeRule}, wantErr: ErrOutboundInUse},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, pool := resourcePostgresPool(t)
			exec := func(sql string, args ...any) {
				t.Helper()
				if _, err := pool.Exec(ctx, sql, args...); err != nil {
					t.Fatal(err)
				}
			}
			exec(`INSERT INTO outbounds(id,name,tag,protocol) VALUES('out_target','Target','target','socks')`)
			if test.proxyReference {
				exec(`INSERT INTO outbounds(id,name,tag,protocol,proxy_tag) VALUES('out_child','Child','child','socks','TARGET')`)
			}
			if test.policyStatus != "" {
				defaultTag := ""
				if test.defaultReference {
					defaultTag = "TARGET"
				}
				exec(`INSERT INTO route_policies(id,name,status,default_outbound_tag,current_revision)
					VALUES('rte_test','Policy',$1,$2,$3)`, test.policyStatus, defaultTag, len(test.revisions))
				for index, rules := range test.revisions {
					exec(`INSERT INTO route_policy_revisions(id,route_policy_id,revision,rules)
						VALUES($1,'rte_test',$2,$3)`, domain.MustID("rrv"), index+1, rules)
				}
			}
			exec(`INSERT INTO machines(id,name) VALUES('mch_test','Machine')`)
			exec(`INSERT INTO nodes(id,machine_id,name,protocol,server_port,status,current_revision) VALUES
				('nod_live','mch_test','Live','socks',1080,'published',8),
				('nod_draft','mch_test','Draft','socks',1081,'draft',8)`)

			s := New(pool)
			if err := s.ArchiveOutbound(ctx, "out_target"); !errors.Is(err, test.wantErr) {
				t.Fatalf("ArchiveOutbound error = %v, want %v", err, test.wantErr)
			}
			wantStatus, wantRevision, wantChanges := "archived", 9, 1
			if test.wantErr != nil {
				wantStatus, wantRevision, wantChanges = "active", 8, 0
			}
			var status string
			if err := pool.QueryRow(ctx, `SELECT status FROM outbounds WHERE id='out_target'`).Scan(&status); err != nil || status != wantStatus {
				t.Fatalf("outbound status = %q, %v; want %q", status, err, wantStatus)
			}
			items, err := s.ListOutbounds(ctx)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, item := range items {
				found = found || item.ID == "out_target"
			}
			if found != (test.wantErr != nil) {
				t.Fatalf("target present in list = %v after archive error %v", found, test.wantErr)
			}
			var revision, changes int
			if err := pool.QueryRow(ctx, `SELECT current_revision FROM nodes WHERE id='nod_live'`).Scan(&revision); err != nil || revision != wantRevision {
				t.Fatalf("live revision = %d, %v; want %d", revision, err, wantRevision)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM control_changes WHERE node_id='nod_live' AND event_type='node.spec.replace'`).Scan(&changes); err != nil || changes != wantChanges {
				t.Fatalf("control changes = %d, %v; want %d", changes, err, wantChanges)
			}
			if err := pool.QueryRow(ctx, `SELECT current_revision FROM nodes WHERE id='nod_draft'`).Scan(&revision); err != nil || revision != 8 {
				t.Fatalf("draft revision = %d, %v; want 8", revision, err)
			}
			if test.wantErr == nil {
				if err := s.ArchiveOutbound(ctx, "out_target"); !errors.Is(err, ErrNotFound) {
					t.Fatalf("repeat archive error = %v, want ErrNotFound", err)
				}
			}
			if err := s.ArchiveOutbound(ctx, "out_missing"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("missing archive error = %v, want ErrNotFound", err)
			}
		})
	}
}

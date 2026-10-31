package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/database"
	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func agentSecurityStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	url := os.Getenv("CPANEL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set CPANEL_TEST_DATABASE_URL to test Agent authentication on PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := domain.MustID("agent_security_test")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		admin.Close()
	})
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return New(pool), ctx
}

func TestAgentCredentialsCannotCrossMachinesOrDowngradeV2Postgres(t *testing.T) {
	s, ctx := agentSecurityStore(t)
	if s.LegacyAgentAllowed(ctx) {
		t.Fatal("fresh database enabled legacy protocol")
	}
	for _, id := range []string{"mch_a", "mch_b", "mch_legacy"} {
		if _, err := s.pool.Exec(ctx, `INSERT INTO machines(id,name) VALUES($1,$1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	shared := auth.HashSecret("test-only-shared-key")
	if _, err := s.pool.Exec(ctx, `INSERT INTO agent_shared_credentials(singleton,token_hash,token_prefix) VALUES(1,$1,'test')`, shared); err != nil {
		t.Fatal(err)
	}
	legacyA, err := s.CreateMachineCredential(ctx, "mch_a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if machine, err := s.AuthenticateAgent(ctx, auth.HashSecret(legacyA.Token)); err != nil || machine.ID != "mch_a" {
		t.Fatalf("machine A authentication: %v", err)
	}
	hashA, hashB := auth.HashSecret("one-use-credential-A"), auth.HashSecret("one-use-credential-B")
	sealed := json.RawMessage(`{"fixture":"encrypted by the HTTP service in production"}`)
	for _, item := range []struct {
		id   string
		hash []byte
	}{{"mch_a", hashA}, {"mch_b", hashB}} {
		if err := s.IssueAgentEnrollment(ctx, item.id, item.hash, sealed, time.Now().Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	public, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := s.EnrollAgentIdentity(ctx, "mch_b", public, hashA); err == nil {
		t.Fatal("A credential enrolled B")
	}
	if _, err := s.AuthenticateSharedAgent(ctx, "mch_b", shared); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending V2 accepted shared key: %v", err)
	}
	if _, err := s.AuthenticateAgent(ctx, auth.HashSecret(legacyA.Token)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending V2 accepted legacy credential: %v", err)
	}
	var group sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		group.Go(func() { _, err := s.EnrollAgentIdentity(ctx, "mch_b", public, hashB); results <- err })
	}
	group.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("one-use credential admitted %d concurrent enrollments", successes)
	}
	if _, _, err := s.AgentEnrollmentCredential(ctx, "mch_b"); !errors.Is(err, ErrNotFound) {
		t.Fatal("used secret was retained")
	}
	if _, err := s.EnrollAgentIdentity(ctx, "mch_b", public, hashB); err == nil {
		t.Fatal("used credential replay accepted")
	}
	// Even if the display flag is accidentally restored, identity binding wins.
	if _, err := s.pool.Exec(ctx, `UPDATE machines SET agent_protocol='legacy' WHERE id='mch_b'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AuthenticateSharedAgent(ctx, "mch_b", shared); !errors.Is(err, ErrNotFound) {
		t.Fatal("identity-bound machine downgraded")
	}
	if err := s.ResetAgentIdentity(ctx, "mch_b"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AuthenticateSharedAgent(ctx, "mch_b", shared); !errors.Is(err, ErrNotFound) {
		t.Fatal("reset enabled legacy downgrade")
	}
	if err := s.IssueAgentEnrollment(ctx, "mch_b", hashB, sealed, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnrollAgentIdentity(ctx, "mch_b", public, hashB); err == nil {
		t.Fatal("expired registration accepted")
	}
	if _, err := s.AuthenticateSharedAgent(ctx, "mch_legacy", shared); err != nil {
		t.Fatalf("unmigrated legacy machine lost compatibility: %v", err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE settings SET value='"invalid"'::jsonb WHERE section='agent' AND key='allow_legacy_protocol'`); err != nil {
		t.Fatal(err)
	}
	if s.LegacyAgentAllowed(ctx) {
		t.Fatal("malformed legacy setting failed open")
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM settings WHERE section='agent' AND key='allow_legacy_protocol'`); err != nil {
		t.Fatal(err)
	}
	if s.LegacyAgentAllowed(ctx) {
		t.Fatal("missing legacy setting failed open")
	}
}

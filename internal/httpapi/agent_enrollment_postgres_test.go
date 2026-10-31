package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"cpanel/internal/agentv2"
	"cpanel/internal/auth"
	"cpanel/internal/database"
	"cpanel/internal/domain"
	"cpanel/internal/securebox"
	"cpanel/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAgentEnrollmentHandshakeLifecyclePostgres(t *testing.T) {
	dsn := os.Getenv("CPANEL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CPANEL_TEST_DATABASE_URL for the real PostgreSQL handshake regression")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := domain.MustID("enrollment_test")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"mch_handshake_a", "mch_handshake_b"} {
		if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name) VALUES($1,$1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	box, err := securebox.New(base64.RawStdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	s := store.New(pool)
	service := newAgentV2Service(s, box)
	pin, err := service.panelPublicKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(id string, expires time.Time) string {
		t.Helper()
		token, hash, err := auth.NewSecret("cpenroll_", 32)
		if err != nil {
			t.Fatal(err)
		}
		plain, _ := json.Marshal(token)
		sealed, err := box.Seal(plain)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.IssueAgentEnrollment(ctx, id, hash, sealed, expires); err != nil {
			t.Fatal(err)
		}
		return token
	}
	public, private, err := agentv2.GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}
	handshake := func(id, token string, enrolled bool) *agentv2.ClientHandshake {
		t.Helper()
		h, err := agentv2.NewClientHandshake(id, public, private, []byte(token), enrolled, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	accept := func(h *agentv2.ClientHandshake) {
		t.Helper()
		response, err := service.acceptHandshake(ctx, h.Request, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.Complete(response, pin); err != nil {
			t.Fatal(err)
		}
	}
	token := issue("mch_handshake_a", time.Now().Add(time.Minute))
	issue("mch_handshake_b", time.Now().Add(time.Minute))
	if _, err := service.acceptHandshake(ctx, handshake("mch_handshake_b", token, false).Request, time.Now()); err == nil {
		t.Fatal("machine A enrollment credential authenticated machine B")
	}
	accept(handshake("mch_handshake_a", token, false))
	if _, _, err := s.AgentEnrollmentCredential(ctx, "mch_handshake_a"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("one-use credential not consumed: %v", err)
	}
	if _, err := service.acceptHandshake(ctx, handshake("mch_handshake_a", token, false).Request, time.Now()); err == nil {
		t.Fatal("consumed enrollment proof accepted again")
	}
	connect := handshake("mch_handshake_a", "", true)
	accept(connect)
	if _, err := service.acceptHandshake(ctx, connect.Request, time.Now()); err == nil {
		t.Fatal("signed handshake replay accepted")
	}
	// Reset requires a newly issued machine-bound credential even if the Agent
	// still has the same private identity file. A restart alone cannot re-enroll.
	if err := s.ResetAgentIdentity(ctx, "mch_handshake_a"); err != nil {
		t.Fatal(err)
	}
	service.deleteMachineSessions("mch_handshake_a")
	if _, err := service.acceptHandshake(ctx, handshake("mch_handshake_a", token, false).Request, time.Now()); err == nil {
		t.Fatal("reset revived a consumed bootstrap credential")
	}
	expired := issue("mch_handshake_a", time.Now().Add(-time.Minute))
	if _, err := service.acceptHandshake(ctx, handshake("mch_handshake_a", expired, false).Request, time.Now()); err == nil {
		t.Fatal("expired registration accepted")
	}
	fresh := issue("mch_handshake_a", time.Now().Add(time.Minute))
	accept(handshake("mch_handshake_a", fresh, false))
	accept(handshake("mch_handshake_a", "", true))
	identity, err := s.AgentIdentity(ctx, "mch_handshake_a")
	if err != nil || !ed25519.PublicKey(identity.PublicKey).Equal(public) {
		t.Fatalf("identity did not persist: %v", err)
	}
}

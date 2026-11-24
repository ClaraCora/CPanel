package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
)

const legacyTelemetryInsert = `INSERT INTO telemetry_batches(machine_id,idempotency_key) VALUES($1,$2) ON CONFLICT DO NOTHING`
const telemetryTestStream = "t2-0123456789abcdef0123456789abcdef-"

func claimTestReceipt(ctx context.Context, tx pgx.Tx, machine, key string, legacyPanel bool) (bool, error) {
	if legacyPanel {
		tag, err := tx.Exec(ctx, legacyTelemetryInsert, machine, key)
		return tag.RowsAffected() != 0, err
	}
	var accepted bool
	err := tx.QueryRow(ctx, "SELECT claim_telemetry_batch($1,$2)", machine, key).Scan(&accepted)
	return accepted, err
}

func TestTelemetryReceiptCompatibilityPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name) VALUES ('mch_a','A'),('mch_b','B')`); err != nil {
		t.Fatal(err)
	}
	// A receipt written by the old SQL before any bitmap/trigger existed.
	if _, err := pool.Exec(ctx, "ALTER TABLE telemetry_batches DISABLE TRIGGER check_legacy_telemetry_insert"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, legacyTelemetryInsert, "mch_a", telemetryTestStream+"6"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE telemetry_batches ENABLE TRIGGER check_legacy_telemetry_insert"); err != nil {
		t.Fatal(err)
	}
	claim := func(machine, key string, legacy, commit, want bool) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if got, err := claimTestReceipt(ctx, tx, machine, key, legacy); err != nil || got != want {
			t.Fatalf("key %q legacy=%v: accepted=%v err=%v, want %v", key, legacy, got, err, want)
		}
		if commit {
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, legacy := range []bool{true, false} {
		claim("mch_a", telemetryTestStream+"6", legacy, true, false)
	}
	// Alternate panel versions, order, block boundaries and machine namespaces.
	for i, suffix := range []string{"1025", "4", "1", "1024", "9223372036854775807"} {
		legacy := i%2 == 0
		key := telemetryTestStream + suffix
		claim("mch_a", key, legacy, false, true)
		claim("mch_a", key, !legacy, true, true)
		claim("mch_a", key, legacy, true, false)
		claim("mch_b", key, false, true, true)
	}
	for _, key := range []string{"opaque", telemetryTestStream + "0", telemetryTestStream + "01", telemetryTestStream + "9223372036854775808", telemetryTestStream + "2\n", "t2-0123456789ABCDEF0123456789ABCDEF-3"} {
		claim("mch_a", key, false, true, true)
		claim("mch_a", key, true, true, false)
		var exact bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM telemetry_batches WHERE machine_id='mch_a' AND idempotency_key=$1)`, key).Scan(&exact); err != nil || !exact {
			t.Fatalf("opaque receipt %q: exact=%v err=%v", key, exact, err)
		}
	}

	// A retry and new reports can hit the same bitmap row at the same time.
	var group sync.WaitGroup
	var accepted atomic.Int64
	for i := range 32 {
		group.Go(func() {
			key := fmt.Sprintf("%s%d", telemetryTestStream, 10+i%8)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			defer tx.Rollback(ctx)
			ok, err := claimTestReceipt(ctx, tx, "mch_a", key, (i/8)%2 == 0)
			if err != nil {
				t.Error(err)
				return
			}
			if err := tx.Commit(ctx); err != nil {
				t.Error(err)
				return
			}
			if ok {
				accepted.Add(1)
			}
		})
	}
	group.Wait()
	if accepted.Load() != 8 {
		t.Fatalf("concurrent accepted=%d, want 8", accepted.Load())
	}

	var blocks, exact, acceptedRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM generate_series(2049,3072) n WHERE claim_telemetry_batch('mch_a',$1||n::text)`, telemetryTestStream).Scan(&acceptedRows); err != nil || acceptedRows != 1024 {
		t.Fatalf("accepted %d consecutive sequences: %v", acceptedRows, err)
	}
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM telemetry_receipt_blocks WHERE machine_id='mch_a' AND block_number=2),
		(SELECT count(*) FROM telemetry_batches WHERE machine_id='mch_a' AND idempotency_key=$1)`, telemetryTestStream+"2049").Scan(&blocks, &exact); err != nil || blocks != 1 || exact != 0 {
		t.Fatalf("compacted receipts: blocks=%d direct=%d err=%v", blocks, exact, err)
	}
}

func TestCompactedTelemetryRetryAccountingPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	node := seedEntitlements(t, pool, "never", 0, 0)
	s := New(pool)
	user := quotaTestUser(t, s, "bitmap", nil)
	for _, sequence := range []string{"3", "1", "3", "2", "1"} {
		quotaTelemetry(t, s, telemetryTestStream+sequence, node, user.AgentID, 5, 5)
	}
	var used int64
	if err := pool.QueryRow(ctx, `SELECT traffic_used_bytes FROM users WHERE id=$1`, user.ID).Scan(&used); err != nil || used != 30 {
		t.Fatalf("used=%d err=%v, want 30", used, err)
	}
}

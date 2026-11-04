package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/config"
	"cpanel/internal/database"
	"cpanel/internal/domain"
	"cpanel/internal/securebox"
	"cpanel/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func consistencyPostgres(t *testing.T) (*Server, *pgxpool.Pool, http.Handler) {
	t.Helper()
	dsn := os.Getenv("CPANEL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CPANEL_TEST_DATABASE_URL for administrative consistency regressions")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := pgx.Identifier{domain.MustID("admin_consistency_test")}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = strings.Trim(schema, `"`)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO admins(id,email,name,password_hash) VALUES('adm_consistency','admin@example.test','Admin','unused')`); err != nil {
		t.Fatal(err)
	}
	data := store.New(pool)
	if _, err := data.CreateAdminSession(ctx, "adm_consistency", auth.HashSecret("consistency-session"), "consistency-csrf", "", "test", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	box, err := securebox.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	server := New(config.Config{}, data, box)
	server.agentRelease.version = "v2.0.9"
	server.agentRelease.checkedAt = time.Now()
	return server, pool, server.Handler()
}

func consistencyRequest(method, path, body, key, version string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "consistency-csrf")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "consistency-session"})
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	if version != "" {
		request.Header.Set("If-Match", version)
	}
	return request
}

func consistencyCall(handler http.Handler, method, path, body, key, version string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, consistencyRequest(method, path, body, key, version))
	return response
}

func responseRecord(t *testing.T, response *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var envelope struct{ Data map[string]json.RawMessage }
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %d %s: %v", response.Code, response.Body.String(), err)
	}
	return envelope.Data
}

type lostAdminResponse struct{ header http.Header }

func (r *lostAdminResponse) Header() http.Header       { return r.header }
func (r *lostAdminResponse) WriteHeader(int)           {}
func (r *lostAdminResponse) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestAdminCreateRetryAfterLostResponsePostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	body := `{"name":"Created once"}`
	key := "create-lost-response-20261104"
	handler.ServeHTTP(&lostAdminResponse{header: make(http.Header)}, consistencyRequest("POST", "/ca/ht/fwq", body, key, ""))
	response := consistencyCall(handler, "POST", "/ca/ht/fwq", body, key, "")
	if response.Code != http.StatusCreated || response.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("retry response=%d %s", response.Code, response.Body.String())
	}
	var id string
	if err := json.Unmarshal(responseRecord(t, response)["id"], &id); err != nil || id == "" {
		t.Fatalf("retry returned no resource ID: %s", response.Body.String())
	}
	var resources, audits, results int
	if err := pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM machines),(SELECT count(*) FROM audit_events WHERE action='machine.create'),(SELECT count(*) FROM admin_idempotency_results)`).Scan(&resources, &audits, &results); err != nil {
		t.Fatal(err)
	}
	if resources != 1 || audits != 1 || results != 1 {
		t.Fatalf("retry resource/audit/cache counts=%d/%d/%d", resources, audits, results)
	}
	conflict := consistencyCall(handler, "POST", "/ca/ht/fwq", `{"name":"Different intent"}`, key, "")
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "IDEMPOTENCY_CONFLICT") {
		t.Fatalf("reusing key with another body=%d %s", conflict.Code, conflict.Body.String())
	}
}

func TestConcurrentAdminCreatesShareOneResultPostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	responses := make(chan *httptest.ResponseRecorder, 6)
	var workers sync.WaitGroup
	for i := 0; i < 6; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			responses <- consistencyCall(handler, "POST", "/ca/ht/yh/ks/yh", "", "concurrent-quick-create", "")
		}()
	}
	workers.Wait()
	close(responses)
	id := ""
	for response := range responses {
		if response.Code != http.StatusCreated {
			t.Fatalf("concurrent create=%d %s", response.Code, response.Body.String())
		}
		var current string
		_ = json.Unmarshal(responseRecord(t, response)["id"], &current)
		if id != "" && current != id {
			t.Fatalf("one intent returned IDs %s and %s", id, current)
		}
		id = current
	}
	var count int
	var sealed string
	if err := pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM users),sealed_response::text FROM admin_idempotency_results`).Scan(&count, &sealed); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("concurrent quick-create produced %d users", count)
	}
	if strings.Contains(sealed, "subscription_token") || strings.Contains(sealed, "cps_") {
		t.Fatal("idempotency cache contains an unencrypted credential")
	}
}

func TestFailedAuditRollsBackResourceAndIntentPostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	if _, err := pool.Exec(t.Context(), `CREATE FUNCTION fail_audit_for_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$;
		CREATE TRIGGER fail_audit_for_test BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_audit_for_test()`); err != nil {
		t.Fatal(err)
	}
	response := consistencyCall(handler, "POST", "/ca/ht/fwq", `{"name":"Must roll back"}`, "audit-failure-retry", "")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("audit failure returned success: %d %s", response.Code, response.Body.String())
	}
	var resources, intents int
	if err := pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM machines),(SELECT count(*) FROM admin_idempotency_results)`).Scan(&resources, &intents); err != nil {
		t.Fatal(err)
	}
	if resources != 0 || intents != 0 {
		t.Fatalf("audit failure committed resource=%d intent=%d", resources, intents)
	}
	if _, err := pool.Exec(t.Context(), `DROP TRIGGER fail_audit_for_test ON audit_events`); err != nil {
		t.Fatal(err)
	}
	retry := consistencyCall(handler, "POST", "/ca/ht/fwq", `{"name":"Must roll back"}`, "audit-failure-retry", "")
	if retry.Code != http.StatusCreated {
		t.Fatalf("retry after audit recovery=%d %s", retry.Code, retry.Body.String())
	}
}

func TestAuditEncodingFailurePoisonsOuterTransactionPostgres(t *testing.T) {
	server, pool, _ := consistencyPostgres(t)
	ctx, transaction, err := server.store.BeginRequest(t.Context(), pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(t.Context())
	if _, err := server.store.CreateMachine(ctx, domain.MachineCreate{Name: "Uncommitted"}); err != nil {
		t.Fatal(err)
	}
	_ = server.store.WriteAudit(ctx, "adm_consistency", "machine.create", "machine", "x", make(chan int), "", "req_test")
	if err := transaction.Commit(ctx); err == nil {
		t.Fatal("ignored audit encoding error allowed commit")
	}
	if err := transaction.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM machines`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("audit encoding failure left %d machines, err=%v", count, err)
	}
}

func TestAdminStaleEditAndRuntimeUpdatesPostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	created := consistencyCall(handler, "POST", "/ca/ht/fwq", `{"name":"Original"}`, "versioned-machine", "")
	var id string
	var version int64
	record := responseRecord(t, created)
	_ = json.Unmarshal(record["id"], &id)
	_ = json.Unmarshal(record["config_version"], &version)
	if version != 1 {
		t.Fatalf("created config version=%d response=%s", version, created.Body.String())
	}
	if _, err := pool.Exec(t.Context(), `UPDATE machines SET agent_version='v3',last_heartbeat_at=now(),status='online',kernel_type='xray',capabilities='{"telemetry":true}' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	path := "/ca/ht/fwq/" + id
	first := consistencyCall(handler, "PATCH", path, `{"name":"First editor"}`, "", `"1"`)
	if first.Code != http.StatusOK {
		t.Fatalf("runtime update caused false edit conflict=%d %s", first.Code, first.Body.String())
	}
	second := consistencyCall(handler, "PATCH", path, `{"name":"Stale editor"}`, "", `"1"`)
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), "EDIT_VERSION_CONFLICT") {
		t.Fatalf("stale editor overwrote first save=%d %s", second.Code, second.Body.String())
	}
	var name string
	if err := pool.QueryRow(t.Context(), `SELECT name FROM machines WHERE id=$1`, id).Scan(&name); err != nil || name != "First editor" {
		t.Fatalf("stored name=%s err=%v", name, err)
	}
}

func TestSevenResourceDeletesRejectStaleVersionsPostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_delete_guard','Machine');
		INSERT INTO nodes(id,machine_id,name,protocol,server_port) VALUES('nod_delete_guard','mch_delete_guard','Node','socks',1080);
		INSERT INTO access_groups(id,name) VALUES('grp_delete_guard','Group');
		INSERT INTO plans(id,access_group_id,name) VALUES('pln_delete_guard','grp_delete_guard','Plan');
		INSERT INTO users(id,name,uuid,subscription_token_hash,subscription_token_prefix) VALUES('usr_delete_guard','User','00000000-0000-4000-8000-000000000009','\x09','prefix');
		INSERT INTO route_policies(id,name) VALUES('rte_delete_guard','Route');
		INSERT INTO outbounds(id,name,tag,protocol) VALUES('out_delete_guard','Outbound','delete-guard','socks')`); err != nil {
		t.Fatal(err)
	}
	// Keep this order so the subsequent successful deletes respect references.
	resources := []struct{ table, path, id string }{
		{"users", "yh", "usr_delete_guard"},
		{"plans", "tc", "pln_delete_guard"},
		{"access_groups", "qxz", "grp_delete_guard"},
		{"nodes", "jd", "nod_delete_guard"},
		{"machines", "fwq", "mch_delete_guard"},
		{"route_policies", "ly", "rte_delete_guard"},
		{"outbounds", "ck", "out_delete_guard"},
	}
	for _, resource := range resources {
		if _, err := pool.Exec(t.Context(), `UPDATE `+resource.table+` SET name=name||' edited' WHERE id=$1`, resource.id); err != nil {
			t.Fatal(err)
		}
	}
	for _, resource := range resources {
		t.Run(resource.table, func(t *testing.T) {
			stale := consistencyCall(handler, "DELETE", "/ca/ht/"+resource.path+"/"+resource.id, "", "", `"1"`)
			if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "EDIT_VERSION_CONFLICT") {
				t.Fatalf("stale delete=%d %s", stale.Code, stale.Body.String())
			}
			var status string
			var version int64
			if err := pool.QueryRow(t.Context(), `SELECT status,config_version FROM `+resource.table+` WHERE id=$1`, resource.id).Scan(&status, &version); err != nil {
				t.Fatal(err)
			}
			if status == "archived" || version != 2 {
				t.Fatalf("stale delete mutated record: status=%s version=%d", status, version)
			}
		})
	}
	var audits, changes int
	if err := pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM audit_events),(SELECT count(*) FROM control_changes)`).Scan(&audits, &changes); err != nil {
		t.Fatal(err)
	}
	if audits != 0 || changes != 0 {
		t.Fatalf("rejected deletes produced audit/control side effects=%d/%d", audits, changes)
	}
	for _, resource := range resources {
		response := consistencyCall(handler, "DELETE", "/ca/ht/"+resource.path+"/"+resource.id, "", "", `"2"`)
		if response.Code != http.StatusOK {
			t.Fatalf("current %s delete=%d %s", resource.table, response.Code, response.Body.String())
		}
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_legacy_delete','Legacy client')`); err != nil {
		t.Fatal(err)
	}
	legacy := consistencyCall(handler, "DELETE", "/ca/ht/fwq/mch_legacy_delete", "", "", "")
	if legacy.Code != http.StatusOK {
		t.Fatalf("delete without If-Match lost compatibility=%d %s", legacy.Code, legacy.Body.String())
	}
}

func TestLogoutStillRevokesSessionWhenAuditFailsPostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	if _, err := pool.Exec(t.Context(), `CREATE FUNCTION fail_logout_audit_for_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$;
		CREATE TRIGGER fail_logout_audit_for_test BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_logout_audit_for_test()`); err != nil {
		t.Fatal(err)
	}
	response := consistencyCall(handler, "DELETE", "/ca/ht/hh/dq", "", "", "")
	if response.Code != http.StatusOK {
		t.Fatalf("audit outage prevented logout=%d %s", response.Code, response.Body.String())
	}
	expired := false
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.MaxAge < 0 {
			expired = true
		}
	}
	if !expired {
		t.Fatal("logout did not clear the browser session cookie")
	}
	var revoked bool
	if err := pool.QueryRow(t.Context(), `SELECT revoked_at IS NOT NULL FROM admin_sessions WHERE token_hash=$1`, auth.HashSecret("consistency-session")).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("logout revocation rolled back: revoked=%v err=%v", revoked, err)
	}
	next := consistencyCall(handler, "GET", "/ca/ht/hh", "", "", "")
	if next.Code != http.StatusUnauthorized {
		t.Fatalf("revoked cookie still authenticated=%d %s", next.Code, next.Body.String())
	}
}

type consistencyTelegramSender struct{ sent int }

func (s *consistencyTelegramSender) SendTest(context.Context) error {
	s.sent++
	return nil
}

func TestTelegramSendDoesNotBecomeFailedAfterAuditFailurePostgres(t *testing.T) {
	server, pool, handler := consistencyPostgres(t)
	sender := &consistencyTelegramSender{}
	server.telegramBot = sender
	if _, err := pool.Exec(t.Context(), `CREATE FUNCTION fail_telegram_audit_for_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$;
		CREATE TRIGGER fail_telegram_audit_for_test BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_telegram_audit_for_test()`); err != nil {
		t.Fatal(err)
	}
	response := consistencyCall(handler, "POST", "/ca/ht/sz/tg/cs", "", "", "")
	if response.Code != http.StatusOK || sender.sent != 1 {
		t.Fatalf("already sent message reported failed: sends=%d response=%d %s", sender.sent, response.Code, response.Body.String())
	}
}

func TestStandaloneAdminOperationsDoNotStartConfigurationTransaction(t *testing.T) {
	server := &Server{}
	handler := server.adminConsistency(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, route := range []struct{ method, path string }{
		{"DELETE", "/ca/ht/hh/dq"},
		{"POST", "/ca/ht/sz/tg/cs"},
		{"POST", "/ca/ht/gj/xsmy"},
		{"POST", "/ca/ht/yh/fwjl/gs"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(route.method, route.path, nil))
		if response.Code != http.StatusNoContent {
			t.Errorf("standalone %s %s unexpectedly opened a configuration transaction", route.method, route.path)
		}
	}
}

func TestPanelIdentityCacheOnlyChangesAfterCommitPostgres(t *testing.T) {
	server, pool, handler := consistencyPostgres(t)
	server.cfg.ExternalURL = "https://panel.example.test"
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_identity_commit','Machine');
		CREATE FUNCTION fail_identity_audit_for_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$;
		CREATE TRIGGER fail_identity_audit_for_test BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_identity_audit_for_test()`); err != nil {
		t.Fatal(err)
	}
	failed := consistencyCall(handler, "POST", "/ca/ht/fwq/mch_identity_commit/az", "", "", "")
	if failed.Code != http.StatusInternalServerError {
		t.Fatalf("installation with failed audit=%d %s", failed.Code, failed.Body.String())
	}
	var identities, credentials int
	if err := pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM agent_panel_identity),(SELECT count(*) FROM agent_enrollment_credentials)`).Scan(&identities, &credentials); err != nil {
		t.Fatal(err)
	}
	if identities != 0 || credentials != 0 || len(server.agentV2.identity) != 0 {
		t.Fatalf("rolled-back installation leaked identity/cache: identities=%d credentials=%d cache=%d", identities, credentials, len(server.agentV2.identity))
	}
	if _, err := pool.Exec(t.Context(), `DROP TRIGGER fail_identity_audit_for_test ON audit_events`); err != nil {
		t.Fatal(err)
	}
	success := consistencyCall(handler, "POST", "/ca/ht/fwq/mch_identity_commit/az", "", "", "")
	if success.Code != http.StatusOK {
		t.Fatalf("installation after recovery=%d %s", success.Code, success.Body.String())
	}
	var stored []byte
	if err := pool.QueryRow(t.Context(), `SELECT public_key FROM agent_panel_identity WHERE singleton=1`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(server.agentV2.identity) != ed25519.PrivateKeySize || !bytes.Equal(server.agentV2.identity.Public().(ed25519.PublicKey), stored) {
		t.Fatal("committed panel identity differs from in-memory cache")
	}
}

func TestIdentityResetPreservesSessionsUntilCommitPostgres(t *testing.T) {
	server, pool, handler := consistencyPostgres(t)
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_reset_commit','Machine');
		INSERT INTO agent_identities(machine_id,public_key) VALUES('mch_reset_commit',decode(repeat('01',32),'hex'));
		CREATE FUNCTION fail_reset_audit_for_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit unavailable'; END $$;
		CREATE TRIGGER fail_reset_audit_for_test BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION fail_reset_audit_for_test()`); err != nil {
		t.Fatal(err)
	}
	server.agentV2.sessions["existing-session"] = agentV2SessionRecord{machine: domain.AgentMachine{ID: "mch_reset_commit"}}
	failed := consistencyCall(handler, "DELETE", "/ca/ht/fwq/mch_reset_commit/sf", "", "", "")
	if failed.Code != http.StatusInternalServerError {
		t.Fatalf("identity reset with failed audit=%d %s", failed.Code, failed.Body.String())
	}
	if _, ok := server.agentV2.sessions["existing-session"]; !ok {
		t.Fatal("rolled-back identity reset removed an active in-memory session")
	}
	if _, err := server.store.AgentIdentity(t.Context(), "mch_reset_commit"); err != nil {
		t.Fatalf("rolled-back identity reset removed persisted identity: %v", err)
	}
	if _, err := pool.Exec(t.Context(), `DROP TRIGGER fail_reset_audit_for_test ON audit_events`); err != nil {
		t.Fatal(err)
	}
	success := consistencyCall(handler, "DELETE", "/ca/ht/fwq/mch_reset_commit/sf", "", "", "")
	if success.Code != http.StatusOK {
		t.Fatalf("identity reset after recovery=%d %s", success.Code, success.Body.String())
	}
	if _, ok := server.agentV2.sessions["existing-session"]; ok {
		t.Fatal("committed identity reset retained an in-memory session")
	}
}

func TestSettingsSectionOptimisticConcurrencyPostgres(t *testing.T) {
	_, _, handler := consistencyPostgres(t)
	response := consistencyCall(handler, "GET", "/ca/ht/sz/zd", "", "", "")
	var envelope struct {
		Meta struct {
			Version int64 `json:"config_version"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || response.Code != http.StatusOK || envelope.Meta.Version < 1 {
		t.Fatalf("settings version missing=%d %s err=%v", response.Code, response.Body.String(), err)
	}
	version := `"` + strconv.FormatInt(envelope.Meta.Version, 10) + `"`
	first := consistencyCall(handler, "PATCH", "/ca/ht/sz/zd", `{"values":{"platform_name":"New name"}}`, "", version)
	if first.Code != http.StatusOK {
		t.Fatalf("settings save=%d %s", first.Code, first.Body.String())
	}
	stale := consistencyCall(handler, "PATCH", "/ca/ht/sz/zd", `{"values":{"platform_name":"Old tab"}}`, "", version)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "EDIT_VERSION_CONFLICT") {
		t.Fatalf("stale settings save=%d %s", stale.Code, stale.Body.String())
	}
}

func TestResourceDataAndVersionShareOneSnapshotPostgres(t *testing.T) {
	server, pool, _ := consistencyPostgres(t)
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_snapshot','Before')`); err != nil {
		t.Fatal(err)
	}
	handler := server.adminConsistency(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items, err := server.store.ListMachines(r.Context())
		if err != nil {
			writeStoreError(w, r, err)
			return
		}
		// Commit a competing edit after the data query but before version lookup.
		if _, err := pool.Exec(r.Context(), `UPDATE machines SET name='After' WHERE id='mch_snapshot'`); err != nil {
			writeStoreError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, items)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/ca/ht/fwq", nil))
	var envelope struct {
		Data []struct {
			Name    string `json:"name"`
			Version int64  `json:"config_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || response.Code != http.StatusOK {
		t.Fatalf("snapshot response=%d %s err=%v", response.Code, response.Body.String(), err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].Name != "Before" || envelope.Data[0].Version != 1 {
		t.Fatalf("mixed record/version snapshots: %+v", envelope.Data)
	}
}

func TestNodePublishReplayAndDraftVersionPostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_node_version','Node machine')`); err != nil {
		t.Fatal(err)
	}
	created := consistencyCall(handler, "POST", "/ca/ht/jd", `{"name":"Original","machine_id":"mch_node_version","protocol":"socks","server_port":1080,"config":{}}`, "versioned-node-create", "")
	if created.Code != http.StatusCreated {
		t.Fatalf("create node=%d %s", created.Code, created.Body.String())
	}
	var id string
	var version int64
	record := responseRecord(t, created)
	_ = json.Unmarshal(record["id"], &id)
	_ = json.Unmarshal(record["config_version"], &version)
	versionHeader := `"` + strconv.FormatInt(version, 10) + `"`
	path := "/ca/ht/jd/" + id
	first := consistencyCall(handler, "POST", path+"/fb", "", "publish-node-once", versionHeader)
	if first.Code != http.StatusOK {
		t.Fatalf("publish node=%d %s", first.Code, first.Body.String())
	}
	// A retry carries the pre-publish If-Match. Replay must run before that check.
	replay := consistencyCall(handler, "POST", path+"/fb", "", "publish-node-once", versionHeader)
	if replay.Code != http.StatusOK || replay.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("publish replay=%d %s", replay.Code, replay.Body.String())
	}
	var revision int
	if err := pool.QueryRow(t.Context(), `SELECT current_revision FROM nodes WHERE id=$1`, id).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("publish retry changed revision=%d err=%v", revision, err)
	}
	_ = json.Unmarshal(responseRecord(t, first)["config_version"], &version)
	versionHeader = `"` + strconv.FormatInt(version, 10) + `"`
	draft := consistencyCall(handler, "PATCH", path, `{"name":"Pending edit"}`, "", versionHeader)
	if draft.Code != http.StatusOK {
		t.Fatalf("draft save=%d %s", draft.Code, draft.Body.String())
	}
	var draftVersion int64
	_ = json.Unmarshal(responseRecord(t, draft)["config_version"], &draftVersion)
	if draftVersion <= version {
		t.Fatalf("draft child did not advance parent edit version: %d -> %d", version, draftVersion)
	}
	stale := consistencyCall(handler, "PATCH", path, `{"name":"Old form"}`, "", versionHeader)
	if stale.Code != http.StatusConflict {
		t.Fatalf("draft overwrite was not rejected=%d %s", stale.Code, stale.Body.String())
	}
	restored := consistencyCall(handler, "POST", path+"/hf", `{"revision":1}`, "restore-node-once", `"`+strconv.FormatInt(draftVersion, 10)+`"`)
	if restored.Code != http.StatusOK {
		t.Fatalf("restore draft=%d %s", restored.Code, restored.Body.String())
	}
	restoredAgain := consistencyCall(handler, "POST", path+"/hf", `{"revision":1}`, "restore-node-once", `"`+strconv.FormatInt(draftVersion, 10)+`"`)
	if restoredAgain.Code != http.StatusOK || restoredAgain.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("restore replay=%d %s", restoredAgain.Code, restoredAgain.Body.String())
	}
}

func TestVersionGuardAcquiresGlobalLockBeforeNodeRowPostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_lock_order','Machine');
		INSERT INTO route_policies(id,name,status,current_revision) VALUES('rte_lock_order','Policy','published',1);
		INSERT INTO route_policy_revisions(id,route_policy_id,revision,rules) VALUES('rr_lock_order','rte_lock_order',1,'[]');
		INSERT INTO nodes(id,machine_id,route_policy_id,name,protocol,server_port,status,current_revision) VALUES('nod_lock_order','mch_lock_order','rte_lock_order','Node','socks',1080,'published',1)`); err != nil {
		t.Fatal(err)
	}
	policy, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer policy.Rollback(t.Context())
	if _, err := policy.Exec(t.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('cpanel:resource-consistency',0))`); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.Exec(t.Context(), `UPDATE route_policies SET notes='Concurrent policy change' WHERE id='rte_lock_order'`); err != nil {
		t.Fatal(err)
	}
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		result <- consistencyCall(handler, "PATCH", "/ca/ht/jd/nod_lock_order", `{"name":"Concurrent node draft"}`, "", `"1"`)
	}()
	select {
	case response := <-result:
		t.Fatalf("node writer bypassed policy transaction: %d %s", response.Code, response.Body.String())
	case <-time.After(100 * time.Millisecond):
	}
	// A policy operation holds the global lock then touches bound nodes. If the
	// middleware locked the node before waiting for global, this would deadlock.
	bounded, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := policy.Exec(bounded, `UPDATE nodes SET current_revision=current_revision+1 WHERE id='nod_lock_order'`); err != nil {
		t.Fatalf("node/policy lock order is inverted: %v", err)
	}
	if err := policy.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	response := <-result
	if response.Code != http.StatusOK {
		t.Fatalf("concurrent node draft failed: %d %s", response.Code, response.Body.String())
	}
}

func TestSevenResourceListsExposeEditVersionsPostgres(t *testing.T) {
	_, pool, handler := consistencyPostgres(t)
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_seven','Machine');
		INSERT INTO nodes(id,machine_id,name,protocol,server_port) VALUES('nod_seven','mch_seven','Node','socks',1080);
		INSERT INTO access_groups(id,name) VALUES('grp_seven','Group');
		INSERT INTO access_group_nodes(access_group_id,node_id) VALUES('grp_seven','nod_seven');
		INSERT INTO plans(id,access_group_id,name) VALUES('pln_seven','grp_seven','Plan');
		INSERT INTO users(id,plan_id,name,uuid,subscription_token_hash,subscription_token_prefix) VALUES('usr_seven','pln_seven','User','00000000-0000-4000-8000-000000000001','\x01','prefix');
		INSERT INTO route_policies(id,name) VALUES('rte_seven','Route');
		INSERT INTO outbounds(id,name,tag,protocol) VALUES('out_seven','Outbound','seven','socks')`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"fwq", "jd", "qxz", "tc", "yh", "ly", "ck"} {
		response := consistencyCall(handler, "GET", "/ca/ht/"+path, "", "", "")
		var envelope struct {
			Data []struct {
				ID      string `json:"id"`
				Version int64  `json:"config_version"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || response.Code != http.StatusOK {
			t.Fatalf("%s list=%d %s err=%v", path, response.Code, response.Body.String(), err)
		}
		if len(envelope.Data) == 0 {
			t.Fatalf("%s list unexpectedly empty", path)
		}
		for _, item := range envelope.Data {
			if item.ID == "" || item.Version < 1 {
				t.Errorf("%s missing ID/version: %+v", path, item)
			}
		}
	}
}

func TestUserVersionGuardPreservesTelemetryLockOrderPostgres(t *testing.T) {
	server, pool, handler := consistencyPostgres(t)
	first, err := server.store.CreateUser(t.Context(), domain.UserCreate{Name: "First traffic user"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := server.store.CreateUser(t.Context(), domain.UserCreate{Name: "Second traffic user"})
	if err != nil {
		t.Fatal(err)
	}
	telemetry, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer telemetry.Rollback(t.Context())
	if _, err := telemetry.Exec(t.Context(), `SELECT id FROM users WHERE id=$1 FOR UPDATE`, first.ID); err != nil {
		t.Fatal(err)
	}
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		result <- consistencyCall(handler, "PATCH", "/ca/ht/yh/"+second.ID, `{"name":"Edited second user"}`, "", `"1"`)
	}()
	select {
	case response := <-result:
		t.Fatalf("edit bypassed traffic lock order: %d %s", response.Code, response.Body.String())
	case <-time.After(100 * time.Millisecond):
	}
	bounded, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if _, err := telemetry.Exec(bounded, `UPDATE users SET traffic_used_bytes=20 WHERE id=$1`, second.ID); err != nil {
		t.Fatalf("single-user version lock inverted telemetry order: %v", err)
	}
	if err := telemetry.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	response := <-result
	if response.Code != http.StatusOK {
		t.Fatalf("user save after telemetry=%d %s", response.Code, response.Body.String())
	}
}

func TestRuntimeColumnsAndChildBindingsHaveCorrectVersionsPostgres(t *testing.T) {
	server, pool, _ := consistencyPostgres(t)
	user, err := server.store.CreateUser(t.Context(), domain.UserCreate{Name: "Runtime user"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO machines(id,name) VALUES('mch_runtime_version','Machine');
		INSERT INTO nodes(id,machine_id,name,protocol,server_port,status,current_revision) VALUES('nod_runtime_version','mch_runtime_version','Node','socks',1080,'published',1);
		INSERT INTO access_groups(id,name) VALUES('grp_runtime_version','Group');
		UPDATE nodes SET applied_revision=1,last_report_at=now(),last_error='Runtime error',current_revision=2 WHERE id='nod_runtime_version'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE users SET traffic_used_bytes=200,traffic_reset_at=now() WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	versions, err := server.store.ResourceConfigVersions(t.Context(), "nodes", []string{"nod_runtime_version"})
	if err != nil || versions["nod_runtime_version"] != 1 {
		t.Fatalf("node runtime changed edit version=%v err=%v", versions, err)
	}
	versions, err = server.store.ResourceConfigVersions(t.Context(), "users", []string{user.ID})
	if err != nil || versions[user.ID] != 1 {
		t.Fatalf("usage changed user edit version=%v err=%v", versions, err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO node_endpoints(id,node_id,name,host,port) VALUES('ep_runtime_version','nod_runtime_version','Endpoint','example.test',443);
		INSERT INTO access_group_nodes(access_group_id,node_id) VALUES('grp_runtime_version','nod_runtime_version')`); err != nil {
		t.Fatal(err)
	}
	versions, err = server.store.ResourceConfigVersions(t.Context(), "nodes", []string{"nod_runtime_version"})
	if err != nil || versions["nod_runtime_version"] != 2 {
		t.Fatalf("endpoint did not advance node version=%v err=%v", versions, err)
	}
	versions, err = server.store.ResourceConfigVersions(t.Context(), "access_groups", []string{"grp_runtime_version"})
	if err != nil || versions["grp_runtime_version"] != 2 {
		t.Fatalf("group binding did not advance group version=%v err=%v", versions, err)
	}
}

func TestEditVersionInputRejectsAmbiguousConditions(t *testing.T) {
	for _, invalid := range []string{"*", `W/"1"`, `"1","2"`, "0", "-1", "abc"} {
		if _, err := parseEditVersion(invalid); err == nil {
			t.Errorf("accepted invalid edit condition %q", invalid)
		}
	}
	if _, err := parseEditVersion(`"12"`); err != nil {
		t.Fatal(err)
	}
}

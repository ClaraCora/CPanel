package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func seedEntitlements(t *testing.T, pool *pgxpool.Pool, strategy string, quota int64, validDays int) int64 {
	t.Helper()
	ctx := t.Context()
	for _, sql := range []string{
		`INSERT INTO machines(id,name,host,status) VALUES('mch_quota','Quota machine','quota.example','online')`,
		`INSERT INTO nodes(id,machine_id,name,protocol,server_port,status,current_revision) VALUES('nod_quota','mch_quota','Quota node','socks',1080,'published',1)`,
		`INSERT INTO access_groups(id,name) VALUES('grp_quota','Quota group')`,
		`INSERT INTO access_group_nodes(access_group_id,node_id) VALUES('grp_quota','nod_quota')`,
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO plans(id,access_group_id,name,traffic_limit_bytes,reset_strategy,default_valid_days)
		VALUES('pln_quota','grp_quota','Quota plan',$1,$2,$3)`, quota, strategy, validDays); err != nil {
		t.Fatal(err)
	}
	var agentID int64
	if err := pool.QueryRow(ctx, `SELECT agent_id FROM nodes WHERE id='nod_quota'`).Scan(&agentID); err != nil {
		t.Fatal(err)
	}
	return agentID
}

func quotaTestUser(t *testing.T, s *Store, suffix string, override *int64) domain.UserCreated {
	t.Helper()
	planID := "pln_quota"
	user, err := s.CreateUser(t.Context(), domain.UserCreate{Name: "Quota user " + suffix, PlanID: &planID, TrafficLimitOverrideBytes: override})
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func quotaTelemetry(t *testing.T, s *Store, batchID string, nodeID, userID, upload, download int64) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"events": []any{map[string]any{
		"type": "node.telemetry", "node_id": nodeID, "occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
		"data": map[string]any{"traffic": map[string][2]int64{fmt.Sprint(userID): {upload, download}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordTelemetryBatch(t.Context(), "mch_quota", batchID, data); err != nil {
		t.Fatal(err)
	}
}

func TestQuotaAuthorizationLimitsPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	nodeID := seedEntitlements(t, pool, "never", 100, 0)
	s := New(pool)
	zero, fifty := int64(0), int64(50)
	cases := []struct {
		name     string
		used     int64
		override *int64
		allowed  bool
	}{
		{"below", 99, nil, true}, {"exactly", 100, nil, false}, {"over", 200, nil, false},
		{"override exhausted", 50, &fifty, false}, {"override unlimited", 200, &zero, true},
	}
	allowed := make(map[int64]bool)
	for _, test := range cases {
		user := quotaTestUser(t, s, test.name, test.override)
		if _, err := pool.Exec(ctx, `UPDATE users SET traffic_used_bytes=$2 WHERE id=$1`, user.ID, test.used); err != nil {
			t.Fatal(err)
		}
		_, err := s.SubscriptionByUserID(ctx, user.ID)
		if test.allowed && err != nil || !test.allowed && !errors.Is(err, ErrNotFound) {
			t.Errorf("%s subscription error=%v, allowed=%v", test.name, err, test.allowed)
		}
		allowed[user.AgentID] = test.allowed
	}
	users, err := s.AgentNodeUsers(ctx, "mch_quota", nodeID)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if !allowed[user.ID] {
			t.Errorf("exhausted user %d was sent to Agent", user.ID)
		}
		delete(allowed, user.ID)
	}
	for id, want := range allowed {
		if want {
			t.Errorf("eligible user %d missing from Agent", id)
		}
	}
}

func TestQuotaRevokesAndChargesTailTrafficPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	nodeID := seedEntitlements(t, pool, "never", 100, 0)
	s := New(pool)
	user := quotaTestUser(t, s, "tail", nil)
	if _, err := pool.Exec(ctx, `DELETE FROM control_changes`); err != nil {
		t.Fatal(err)
	}
	quotaTelemetry(t, s, "quota-hit", nodeID, user.AgentID, 60, 40)
	users, err := s.AgentNodeUsers(ctx, "mch_quota", nodeID)
	if err != nil || len(users) != 0 {
		t.Fatalf("Agent users after reaching quota=%v, err=%v", users, err)
	}
	quotaTelemetry(t, s, "quota-tail", nodeID, user.AgentID, 20, 30)
	quotaTelemetry(t, s, "quota-tail", nodeID, user.AgentID, 20, 30) // same accepted batch must not count twice
	var used, recorded, changes int64
	if err := pool.QueryRow(ctx, `SELECT u.traffic_used_bytes,(SELECT sum(upload_bytes+download_bytes) FROM traffic_daily WHERE user_id=u.id),
		(SELECT count(*) FROM control_changes WHERE event_type='node.members.replace') FROM users u WHERE u.id=$1`, user.ID).
		Scan(&used, &recorded, &changes); err != nil {
		t.Fatal(err)
	}
	if used != 150 || recorded != 150 || changes != 1 {
		t.Fatalf("after tail used=%d recorded=%d notifications=%d, want 150/150/1", used, recorded, changes)
	}
	higher := int64(200)
	if _, err := s.UpdatePlan(ctx, "pln_quota", domain.PlanUpdate{TrafficLimitBytes: &higher}); err != nil {
		t.Fatal(err)
	}
	users, err = s.AgentNodeUsers(ctx, "mch_quota", nodeID)
	if err != nil || len(users) != 1 {
		t.Fatalf("raising quota did not restore user: users=%v err=%v", users, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM control_changes WHERE event_type='node.members.replace'`).Scan(&changes); err != nil || changes != 2 {
		t.Fatalf("quota recovery notification count=%d err=%v", changes, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status='paused',expires_at=now()-interval '1 minute' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	quotaTelemetry(t, s, "paused-tail", nodeID, user.AgentID, 5, 5)
	if err := pool.QueryRow(ctx, `SELECT traffic_used_bytes FROM users WHERE id=$1`, user.ID).Scan(&used); err != nil || used != 160 {
		t.Fatalf("tail after pause/expiry used=%d err=%v, want 160", used, err)
	}
}

func TestMonthlyResetRestoresQuotaAuthorizationPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	nodeID := seedEntitlements(t, pool, "calendar_month", 100, 0)
	s := New(pool)
	user := quotaTestUser(t, s, "monthly", nil)
	location := loadTrafficLocation("Asia/Shanghai")
	start, _, _ := trafficMonthBounds(time.Now(), location)
	previous, _ := time.Parse(time.DateOnly, start)
	if _, err := pool.Exec(ctx, `INSERT INTO traffic_daily(day,user_id,node_id,upload_bytes) VALUES($1::date,$2,'nod_quota',200)`, previous.AddDate(0, 0, -1).Format(time.DateOnly), user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET traffic_used_bytes=200,traffic_reset_at=$2::date WHERE id=$1`, user.ID, start); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM control_changes`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReconcileTrafficUsage(ctx); err != nil {
		t.Fatal(err)
	}
	users, err := s.AgentNodeUsers(ctx, "mch_quota", nodeID)
	if err != nil || len(users) != 1 {
		t.Fatalf("monthly reset did not restore user: users=%v err=%v", users, err)
	}
	var used, changes int64
	if err := pool.QueryRow(ctx, `SELECT traffic_used_bytes,(SELECT count(*) FROM control_changes) FROM users WHERE id=$1`, user.ID).Scan(&used, &changes); err != nil {
		t.Fatal(err)
	}
	if used != 0 || changes != 1 {
		t.Fatalf("reset used=%d changes=%d, want 0/1", used, changes)
	}
}

func TestPrunePreservesCurrentMonthUsagePostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	seedEntitlements(t, pool, "calendar_month", 1000, 0)
	s := New(pool)
	user := quotaTestUser(t, s, "retention", nil)
	location := loadTrafficLocation("Pacific/Kiritimati")
	start, _, _ := trafficMonthBounds(time.Now(), location)
	previous, _ := time.Parse(time.DateOnly, start)
	if _, err := pool.Exec(ctx, `INSERT INTO settings(section,key,value) VALUES('site','timezone','"Pacific/Kiritimati"'),('retention','traffic_days','1') ON CONFLICT(section,key) DO UPDATE SET value=EXCLUDED.value`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO traffic_daily(day,user_id,node_id,upload_bytes) VALUES($1::date,$2,'nod_quota',200)`,
		`INSERT INTO imported_user_traffic_daily(day,user_id,download_bytes) VALUES($1::date,$2,100)`,
	} {
		if _, err := pool.Exec(ctx, query, start, user.ID); err != nil {
			t.Fatal(err)
		}
	}
	old := previous.AddDate(0, 0, -1).Format(time.DateOnly)
	if _, err := pool.Exec(ctx, `INSERT INTO traffic_daily(day,user_id,node_id,upload_bytes) VALUES($1::date,$2,'nod_quota',999)`, old, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReconcileTrafficUsage(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.PruneHistoricalData(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReconcileTrafficUsage(ctx); err != nil {
		t.Fatal(err)
	}
	var used, remaining int64
	if err := pool.QueryRow(ctx, `SELECT traffic_used_bytes,(SELECT count(*) FROM traffic_daily WHERE day=$2::date) FROM users WHERE id=$1`, user.ID, old).Scan(&used, &remaining); err != nil {
		t.Fatal(err)
	}
	if used != 300 || remaining != 0 {
		t.Fatalf("prune used=%d old details=%d, want 300/0", used, remaining)
	}
}

func TestConcurrentTelemetryAndReconciliationPreserveQuotaPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	nodeID := seedEntitlements(t, pool, "calendar_month", 100, 0)
	s := New(pool)
	user := quotaTestUser(t, s, "concurrent", nil)
	if _, err := pool.Exec(ctx, `DELETE FROM control_changes`); err != nil {
		t.Fatal(err)
	}
	errorsCh := make(chan error, 12)
	var workers sync.WaitGroup
	for i := 0; i < 10; i++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			data, err := json.Marshal(map[string]any{"events": []any{map[string]any{
				"type": "node.telemetry", "node_id": nodeID,
				"data": map[string]any{"traffic": map[string][2]int64{fmt.Sprint(user.AgentID): {10, 10}}},
			}}})
			if err == nil {
				_, err = s.RecordTelemetryBatch(ctx, "mch_quota", fmt.Sprintf("concurrent-%d", index), data)
			}
			errorsCh <- err
		}(i)
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		for i := 0; i < 10; i++ {
			if _, err := s.ReconcileTrafficUsage(ctx); err != nil {
				errorsCh <- err
				return
			}
		}
		errorsCh <- nil
	}()
	workers.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	var used, recorded, notifications int64
	if err := pool.QueryRow(ctx, `SELECT traffic_used_bytes,
		(SELECT sum(upload_bytes+download_bytes) FROM traffic_daily WHERE user_id=$1),
		(SELECT count(*) FROM control_changes WHERE event_type='node.members.replace') FROM users WHERE id=$1`, user.ID).
		Scan(&used, &recorded, &notifications); err != nil {
		t.Fatal(err)
	}
	if used != 200 || recorded != 200 || notifications != 1 {
		t.Fatalf("concurrent accounting used=%d recorded=%d notifications=%d, want 200/200/1", used, recorded, notifications)
	}
}

func TestPlanExpiryAndAssignmentRulesPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	seedEntitlements(t, pool, "never", 0, 30)
	s := New(pool)
	planID := "pln_quota"
	start := time.Now()
	defaultUser := quotaTestUser(t, s, "default expiry", nil)
	if defaultUser.ExpiresAt == nil || defaultUser.ExpiresAt.Before(start.AddDate(0, 0, 30).Add(-time.Second)) || defaultUser.ExpiresAt.After(time.Now().AddDate(0, 0, 30).Add(time.Second)) {
		t.Fatalf("default 30-day expiry=%v", defaultUser.ExpiresAt)
	}
	custom := time.Now().AddDate(0, 0, 10).UTC().Truncate(time.Second)
	for _, test := range []struct {
		mode string
		date *time.Time
		want *time.Time
	}{
		{"custom", &custom, &custom}, {"permanent", nil, nil},
	} {
		created, err := s.CreateUser(ctx, domain.UserCreate{Name: test.mode, PlanID: &planID, ExpiresMode: test.mode, ExpiresAt: test.date})
		if err != nil {
			t.Fatal(err)
		}
		if test.want == nil && created.ExpiresAt != nil || test.want != nil && (created.ExpiresAt == nil || !created.ExpiresAt.Equal(*test.want)) {
			t.Errorf("%s expiry=%v want=%v", test.mode, created.ExpiresAt, test.want)
		}
	}
	if _, err := s.CreateUser(ctx, domain.UserCreate{Name: "bad custom", PlanID: &planID, ExpiresMode: "custom"}); !errors.Is(err, ErrUserExpiryInvalid) {
		t.Fatalf("missing custom date error=%v", err)
	}
	noPlan, err := s.CreateUser(ctx, domain.UserCreate{Name: "no plan"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.UpdateUser(ctx, noPlan.ID, domain.UserUpdate{PlanID: &planID})
	if err != nil || first.ExpiresAt == nil {
		t.Fatalf("first assignment expiry=%v err=%v", first.ExpiresAt, err)
	}
	customWithoutPlan, err := s.CreateUser(ctx, domain.UserCreate{Name: "custom before plan", ExpiresMode: "custom", ExpiresAt: &custom})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := s.UpdateUser(ctx, customWithoutPlan.ID, domain.UserUpdate{PlanID: &planID})
	if err != nil || kept.ExpiresAt == nil || !kept.ExpiresAt.Equal(custom) {
		t.Fatalf("first assignment silently changed explicit expiry=%v err=%v", kept.ExpiresAt, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO plans(id,access_group_id,name,default_valid_days,reset_strategy) VALUES('pln_other','grp_quota','Other',365,'never')`); err != nil {
		t.Fatal(err)
	}
	other := "pln_other"
	kept, err = s.UpdateUser(ctx, defaultUser.ID, domain.UserUpdate{PlanID: &other})
	if err != nil || kept.ExpiresAt == nil || !kept.ExpiresAt.Equal(*defaultUser.ExpiresAt) {
		t.Fatalf("changing plan silently changed expiry=%v err=%v", kept.ExpiresAt, err)
	}
	if _, err := s.UpdateUser(ctx, defaultUser.ID, domain.UserUpdate{PlanID: &planID}); err != nil {
		t.Fatal(err)
	}
	name := "renamed"
	unchanged, err := s.UpdateUser(ctx, defaultUser.ID, domain.UserUpdate{Name: &name})
	if err != nil || unchanged.ExpiresAt == nil || !unchanged.ExpiresAt.Equal(*defaultUser.ExpiresAt) {
		t.Fatalf("ordinary edit changed expiry=%v original=%v err=%v", unchanged.ExpiresAt, defaultUser.ExpiresAt, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE plans SET status='disabled' WHERE id=$1`, planID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, domain.UserCreate{Name: "disabled assignment", PlanID: &planID}); !errors.Is(err, ErrPlanUnavailable) {
		t.Fatalf("new disabled plan assignment error=%v", err)
	}
	if _, err := s.UpdateUser(ctx, defaultUser.ID, domain.UserUpdate{Name: &name, PlanID: &planID}); err != nil {
		t.Fatalf("existing disabled binding blocked ordinary edit: %v", err)
	}
	if _, err := s.SubscriptionByUserID(ctx, defaultUser.ID); err != nil {
		t.Fatalf("disabling plan interrupted old assignment: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE plans SET status='archived' WHERE id=$1`, planID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, domain.UserCreate{Name: "archived assignment", PlanID: &planID}); !errors.Is(err, ErrPlanUnavailable) {
		t.Fatalf("new archived plan assignment error=%v", err)
	}
}

func TestNewAssignmentWaitsForConcurrentPlanDisablePostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	seedEntitlements(t, pool, "never", 0, 0)
	s := New(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE plans SET status='disabled' WHERE id='pln_quota'`); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		planID := "pln_quota"
		_, err := s.CreateUser(ctx, domain.UserCreate{Name: "concurrent assignment", PlanID: &planID})
		result <- err
	}()
	<-started
	select {
	case err := <-result:
		t.Fatalf("assignment bypassed pending plan disable: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, ErrPlanUnavailable) {
		t.Fatalf("concurrent assignment error=%v, want unavailable", err)
	}
}

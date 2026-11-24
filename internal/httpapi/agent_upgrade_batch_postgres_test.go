package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"cpanel/internal/domain"
	"cpanel/internal/store"
)

func TestAgentUpgradeBatchLifecyclePostgres(t *testing.T) {
	s, pool, handler := consistencyPostgres(t)
	ctx := context.Background()
	s.agentRelease.version = "v2.1.0"
	s.agentRelease.checkedAt = time.Now()
	if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name,status,agent_version,last_heartbeat_at) VALUES
		('online','First','online','v2.0.0',now()),('offline','Second','offline','v2.0.0',NULL),
		('current','Current','online','v2.1.0+build',now()),('d','Disabled','disabled','v2.0.0',NULL),
		('a','Archived','archived','v2.0.0',NULL),('pending','Never connected','pending','',NULL)`); err != nil {
		t.Fatal(err)
	}
	if r := consistencyCall(handler, "GET", "/ca/ht/sjpc/zx", "", "", ""); r.Code != 200 || string(r.Body.Bytes()) == "" {
		t.Fatal(r.Result().Status, r.Body.String())
	}
	if r := consistencyCall(handler, "POST", "/ca/ht/sjpc", `{"target_version":"v2.0.0"}`, "old-batch-intent", ""); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	const create = "/ca/ht/sjpc"
	const request = `{"target_version":"v2.1.0"}`
	var group sync.WaitGroup
	responses := make([]*httptest.ResponseRecorder, 2)
	for i := range responses {
		group.Go(func() {
			responses[i] = consistencyCall(handler, "POST", create, request, "test-batch-create-idempotency", "")
		})
	}
	group.Wait()
	read := func(r *httptest.ResponseRecorder, want int) domain.AgentUpgradeBatch {
		t.Helper()
		if r.Code != want {
			t.Fatalf("%d: %s", r.Code, r.Body.String())
		}
		var res struct{ Data domain.AgentUpgradeBatch }
		if err := json.Unmarshal(r.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		return res.Data
	}
	b := read(responses[0], 202)
	b2 := read(responses[1], 202)
	if b.ID == "" || b.ID != b2.ID || len(b.Items) != 4 {
		t.Fatalf("batch %+v, replay %+v", b, b2)
	}
	getItem := func(id string) domain.AgentUpgradeBatchItem {
		t.Helper()
		fresh := read(consistencyCall(handler, "GET", create+"/"+b.ID, "", "", ""), 200)
		for _, i := range fresh.Items {
			if i.MachineID == id {
				return i
			}
		}
		t.Fatalf("missing %s", id)
		return domain.AgentUpgradeBatchItem{}
	}
	for id, expected := range map[string]string{"online": "queued", "offline": "waiting_online", "pending": "waiting_online", "current": "already_current"} {
		if item := getItem(id); item.Status != expected {
			t.Fatalf("%s: %+v", id, item)
		}
	}
	// A fresh Store has no process-local state from the initial request.
	control := store.New(pool)
	heartbeat := func(id, version string) {
		t.Helper()
		if err := control.RecordMachineHeartbeat(ctx, domain.AgentMachine{ID: id}, version, "xray", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	heartbeat("offline", "v2.0.0")
	command, ok, err := control.ClaimMachineAgentUpgrade(ctx, "offline")
	if err != nil || !ok || command.TargetVersion != "v2.1.0" || command.ID != getItem("offline").TaskID {
		t.Fatal(command, ok, err)
	}
	if _, ok, err := control.ClaimMachineAgentUpgrade(ctx, "offline"); err != nil || ok {
		t.Fatal("duplicate dispatch", err)
	}
	if _, err := control.ReportMachineAgentUpgrade(ctx, "offline", command.ID, "succeeded", "v2.0.0", ""); err == nil {
		t.Fatal("old version accepted")
	}
	if _, err := control.ReportMachineAgentUpgrade(ctx, "offline", command.ID, "failed", "", "installer timeout"); err != nil {
		t.Fatal(err)
	}
	if item := getItem("offline"); item.Status != "failed" || item.Error != "installer timeout" {
		t.Fatal(item)
	}
	path := create + "/" + b.ID + "/fwq/offline/cs"
	retry := read(consistencyCall(handler, "POST", path, "{}", "test-batch-retry-1", ""), 202)
	if retry.ID != b.ID {
		t.Fatal(retry)
	}
	_ = read(consistencyCall(handler, "POST", path, "{}", "test-batch-retry-1", ""), 202)
	if r := consistencyCall(handler, "POST", path, "{}", "test-batch-retry-2", ""); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	task, ok, err := control.ClaimMachineAgentUpgrade(ctx, "offline")
	if err != nil || !ok || task.ID == command.ID {
		t.Fatal(task, ok, err)
	}
	heartbeat("offline", "v2.1.0+verified")
	if item := getItem("offline"); item.Status != "succeeded" || item.AgentVersion != "v2.1.0+verified" {
		t.Fatal(item)
	}
	// An offline upgrade carried out elsewhere should not reinstall or issue a command.
	heartbeat("pending", "v2.1.0")
	if _, ok, err := control.ClaimMachineAgentUpgrade(ctx, "pending"); err != nil || ok {
		t.Fatal("already current", err)
	}
	if i := getItem("pending"); i.Status != "succeeded" {
		t.Fatal(i)
	}
	// Disable and re-enable without reversing or redispatching the recorded cancellation.
	if _, err := pool.Exec(ctx, `UPDATE machines SET status='disabled' WHERE id='online'`); err != nil {
		t.Fatal(err)
	}
	if i := getItem("online"); i.Status != "failed" {
		t.Fatal(i)
	}
	if _, err := pool.Exec(ctx, `UPDATE machines SET status='pending' WHERE id='online'`); err != nil {
		t.Fatal(err)
	}
	heartbeat("online", "v2.0.0")
	if _, ok, err := control.ClaimMachineAgentUpgrade(ctx, "online"); err != nil || ok {
		t.Fatal("cancelled task revived", err)
	}
	if latest := read(consistencyCall(handler, "GET", create+"/zx", "", "", ""), http.StatusOK); latest.ID != b.ID {
		t.Fatal(latest)
	}
}

func TestBatchSnapshotSharesCommandsAcrossPagesAndRejectsDowngradePostgres(t *testing.T) {
	s, pool, _ := consistencyPostgres(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name,status,agent_version,last_heartbeat_at)
		SELECT 'machine-'||n,'Machine '||n,'online','v2.0.0',now() FROM generate_series(1,125) n`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE machines SET agent_version='v3.0.0-rc.1+build' WHERE id='machine-1'`); err != nil {
		t.Fatal(err)
	}
	var results [2]*domain.AgentUpgradeBatch
	var failures [2]error
	var group sync.WaitGroup
	for i := range results {
		group.Go(func() { results[i], failures[i] = s.store.CreateAgentUpgradeBatch(ctx, "v2.2.0") })
	}
	group.Wait()
	for index, err := range failures {
		if err != nil {
			t.Fatal(index, err)
		}
	}
	left, right := results[0], results[1]
	if len(left.Items) != 125 || len(right.Items) != 125 {
		t.Fatalf("partial snapshot %d / %d", len(left.Items), len(right.Items))
	}
	for i, item := range left.Items {
		if item.MachineID != right.Items[i].MachineID || item.TaskID != right.Items[i].TaskID {
			t.Fatalf("distinct live commands: %+v %+v", item, right.Items[i])
		}
		if item.MachineID == "machine-1" && (item.Status != "failed" || item.TaskID != "") {
			t.Fatal("newer release scheduled for downgrade", item)
		}
	}
	const id = "machine-2"
	command, ok, err := s.store.ClaimMachineAgentUpgrade(ctx, id)
	if err != nil || !ok {
		t.Fatal(command, ok, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE machines SET agent_upgrade_dispatched_at=now()-interval '20 minutes' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.ReconcileMachinePresence(ctx); err != nil {
		t.Fatal(err)
	}
	status := func(batchID, machine string) string {
		t.Helper()
		batch, err := s.store.AgentUpgradeBatch(ctx, batchID)
		if err != nil {
			t.Fatal(err)
		}
		for _, i := range batch.Items {
			if i.MachineID == machine {
				return i.Status
			}
		}
		t.Fatal("missing snapshot", machine)
		return ""
	}
	for _, b := range results {
		if got := status(b.ID, id); got != "timed_out" {
			t.Fatal(got)
		}
	}
	if _, err := s.store.ReportMachineAgentUpgrade(ctx, id, command.ID, "succeeded", "v2.2.0", ""); err != nil {
		t.Fatal(err)
	}
	for _, b := range results {
		if got := status(b.ID, id); got != "succeeded" {
			t.Fatal(got)
		}
	}
	if err := s.store.RecordMachineHeartbeat(ctx, domain.AgentMachine{ID: "machine-3"}, "v2.3.0+build", "xray", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.store.ClaimMachineAgentUpgrade(ctx, "machine-3"); err != nil || ok {
		t.Fatal("newer reconnect downgraded", err)
	}
	for _, b := range results {
		if got := status(b.ID, "machine-3"); got != "failed" {
			t.Fatal(got)
		}
	}
}

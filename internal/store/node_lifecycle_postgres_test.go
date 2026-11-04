package store

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNodeNewDesiredRevisionClearsPreviousApplicationFailurePostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	s := New(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO admins(id,email,name,password_hash) VALUES('a_lifecycle','lifecycle@example.test','Lifecycle','unused')`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"m_lifecycle_old", "m_lifecycle_new"} {
		if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name) VALUES($1,$1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	policy, err := s.CreateRoutePolicy(ctx, "Lifecycle route", "", "default", "direct", "a_lifecycle", nil)
	if err != nil {
		t.Fatal(err)
	}
	node, err := s.CreateNode(ctx, domain.NodeCreate{MachineID: "m_lifecycle_old", Name: "Lifecycle", Protocol: "socks", ServerPort: 1080, RoutePolicyID: &policy.ID})
	if err != nil {
		t.Fatal(err)
	}
	node, err = s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	send := func(machine string, revision int, status, message string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"events": []any{map[string]any{
			"type": "node.application", "node_id": node.AgentID,
			"data": map[string]any{"revision": revision, "status": status, "stage": "kernel", "error": message},
		}}})
		if _, err := s.RecordTelemetryBatch(ctx, machine, domain.MustID("application"), body); err != nil {
			t.Fatal(err)
		}
	}
	send(node.MachineID, node.CurrentRevision, "applied", "")
	checkPending := func(previous, current int) {
		t.Helper()
		got, err := s.GetNode(ctx, node.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.CurrentRevision != current || got.AppliedRevision != 1 || got.ReceivedRevision != previous || got.AttemptedRevision != previous ||
			got.ApplyStatus != "" || got.ApplyStage != "" || got.LastError != "" || got.LastReport == nil {
			t.Fatalf("new desired revision retained stale result or erased old confirmation: %+v", got)
		}
	}
	// The first edit gets a new desired version; the last confirmed runtime
	// remains version 1 until a separate, explicit application event arrives.
	name := "Second revision"
	if _, err := s.UpdateNode(ctx, node.ID, domain.NodeUpdate{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PublishNode(ctx, node.ID, ""); err != nil {
		t.Fatal(err)
	}
	send("m_lifecycle_old", 2, "failed", "old bind failure")
	name = "Third revision"
	if _, err := s.UpdateNode(ctx, node.ID, domain.NodeUpdate{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PublishNode(ctx, node.ID, ""); err != nil {
		t.Fatal(err)
	}
	checkPending(2, 3)
	// Policy and outbound edits must reset the same presentation state, without
	// manufacturing an ACK or leaking a prior version's error into this one.
	send("m_lifecycle_old", 3, "failed", "old route failure")
	defaultTag := "block"
	if _, err := s.UpdateRoutePolicy(ctx, policy.ID, "a_lifecycle", domain.RoutePolicyUpdate{DefaultOutboundTag: &defaultTag}); err != nil {
		t.Fatal(err)
	}
	checkPending(3, 4)
	send("m_lifecycle_old", 4, "failed", "old outbound failure")
	outbound, err := s.CreateOutbound(ctx, domain.Outbound{Name: "Unused", Tag: "lifecycle-proxy", Protocol: "socks", Settings: json.RawMessage(`{"servers":[{"address":"old.example.test","port":1080}]}`)})
	if err != nil {
		t.Fatal(err)
	}
	settings := json.RawMessage(`{"servers":[{"address":"new.example.test","port":1080}]}`)
	if _, err := s.UpdateOutbound(ctx, outbound.ID, domain.OutboundUpdate{Settings: &settings}); err != nil {
		t.Fatal(err)
	}
	checkPending(4, 5)
	// A confirmation belongs to the machine that applied it. Moving the node
	// must not make its new owner appear to have received/applied the old work.
	send("m_lifecycle_old", 5, "failed", "old owner failure")
	machine := "m_lifecycle_new"
	if _, err := s.UpdateNode(ctx, node.ID, domain.NodeUpdate{MachineID: &machine}); err != nil {
		t.Fatal(err)
	}
	moved, err := s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if moved.CurrentRevision != 6 || moved.MachineID != machine || moved.ReceivedRevision != 0 || moved.AppliedRevision != 0 || moved.AttemptedRevision != 0 ||
		moved.ApplyStatus != "" || moved.ApplyStage != "" || moved.LastError != "" || moved.LastReport != nil {
		t.Fatalf("new owner inherited old owner application state: %+v", moved)
	}
	send("m_lifecycle_old", 6, "applied", "")
	unchanged, err := s.GetNode(ctx, node.ID)
	if err != nil || unchanged.AppliedRevision != 0 {
		t.Fatalf("old owner confirmed moved node: %+v %v", unchanged, err)
	}
	send(machine, 6, "applied", "")
	confirmed, err := s.GetNode(ctx, node.ID)
	if err != nil || confirmed.AppliedRevision != 6 || confirmed.ApplyStatus != "applied" {
		t.Fatalf("new owner could not confirm moved node: %+v %v", confirmed, err)
	}
}

func TestPublishNodeWithoutChangesReleasesTransactionConnectionPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name) VALUES('m_idempotent','Idempotent')`); err != nil {
		t.Fatal(err)
	}
	s := New(pool)
	node, err := s.CreateNode(ctx, domain.NodeCreate{MachineID: "m_idempotent", Name: "Idempotent", Protocol: "socks", ServerPort: 1080})
	if err != nil {
		t.Fatal(err)
	}
	node, err = s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg := pool.Config()
	cfg.MaxConns, cfg.MinConns = 1, 0
	single, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(single.Close)
	retryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	retried, err := New(single).PublishNode(retryCtx, node.ID, "")
	if err != nil {
		t.Fatalf("publishing unchanged node blocked while holding sole connection: %v", err)
	}
	if retried.CurrentRevision != node.CurrentRevision || retried.HasDraft {
		t.Fatalf("unchanged publication created another revision: %+v", retried)
	}
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM control_changes WHERE node_id=$1 AND event_type='node.spec.replace'`, node.ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("unchanged publication duplicated control events: %d %v", events, err)
	}
}

func TestNodeRevisionRestoresCompleteDocumentAndKeepsLiveSpecUntilPublishPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	s := New(pool)
	for _, id := range []string{"m_restore_old", "m_restore_new"} {
		if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name) VALUES($1,$1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	node, err := s.CreateNode(ctx, domain.NodeCreate{MachineID: "m_restore_old", Name: "Original", Protocol: "socks", ListenIP: "127.0.0.1", ServerPort: 1080,
		KernelType: "xray", Config: json.RawMessage(`{"udp":true,"nested":{"keep":"original"}}`),
		Endpoints: []domain.NodeEndpoint{{Name: "Public", Host: "old.example.test", Port: 443, Status: "active", AccessScope: "default"},
			{Name: "Admin", Host: "admin.example.test", Port: 8443, Status: "active", AccessScope: "admin"}}})
	if err != nil {
		t.Fatal(err)
	}
	node, err = s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	original, err := s.AgentNodeSpec(ctx, "m_restore_old", node.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	machine, name, protocol, listen, kernel, port := "m_restore_new", "Changed", "vless", "::", "singbox", 8443
	settings := json.RawMessage(`{"udp":false,"nested":{"keep":"changed"}}`)
	endpoints := []domain.NodeEndpoint{{Name: "Changed", Host: "new.example.test", Port: 8443, Status: "active", AccessScope: "default"}}
	if _, err := s.UpdateNode(ctx, node.ID, domain.NodeUpdate{MachineID: &machine, Name: &name, Protocol: &protocol, ListenIP: &listen,
		KernelType: &kernel, ServerPort: &port, Config: &settings, Endpoints: &endpoints}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PublishNode(ctx, node.ID, ""); err != nil {
		t.Fatal(err)
	}
	current, err := s.AgentNodeSpec(ctx, machine, node.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := s.RestoreNodeRevision(ctx, node.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !draft.HasDraft || draft.RestoredFromRevision != 1 || draft.MachineID != "m_restore_old" || draft.Name != "Original" ||
		draft.Protocol != "socks" || draft.ListenIP != "127.0.0.1" || draft.ServerPort != 1080 || draft.KernelType != "xray" || len(draft.Endpoints) != 2 {
		t.Fatalf("restoration lost document fields: %+v", draft)
	}
	for i, want := range node.Endpoints {
		got := draft.Endpoints[i]
		if got.Name != want.Name || got.Host != want.Host || got.Port != want.Port || got.Status != want.Status || got.AccessScope != want.AccessScope || got.SortOrder != want.SortOrder {
			t.Fatalf("restoration lost endpoint %d: %+v want %+v", i, got, want)
		}
	}
	stillLive, err := s.AgentNodeSpec(ctx, machine, node.AgentID)
	if err != nil || !reflect.DeepEqual(current, stillLive) {
		t.Fatalf("restoring a draft changed live spec: before=%+v after=%+v err=%v", current, stillLive, err)
	}
	if nodes, err := s.AgentNodes(ctx, "m_restore_old"); err != nil || len(nodes) != 0 {
		t.Fatalf("restoring draft moved live ownership: %+v %v", nodes, err)
	}
	restored, err := s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if restored.CurrentRevision != 3 || restored.HasDraft || restored.RestoredFromRevision != 0 {
		t.Fatalf("restored publication state=%+v", restored)
	}
	restoredSpec, err := s.AgentNodeSpec(ctx, "m_restore_old", node.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	original.Revision, restoredSpec.Revision = 0, 0
	if !reflect.DeepEqual(original, restoredSpec) {
		t.Fatalf("restored resolved spec differs: original=%+v restored=%+v", original, restoredSpec)
	}
	if nodes, err := s.AgentNodes(ctx, machine); err != nil || len(nodes) != 0 {
		t.Fatalf("previous owner retained restored node: %+v %v", nodes, err)
	}
}

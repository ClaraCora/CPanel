package store

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"cpanel/internal/domain"
)

func TestNodeDraftDoesNotChangeRunningConfigurationPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	s := New(pool)
	for _, id := range []string{"m_old", "m_new"} {
		if _, err := pool.Exec(ctx, `INSERT INTO machines(id,name,host) VALUES($1,$1,$1||'.example.test')`, id); err != nil {
			t.Fatal(err)
		}
	}
	node, err := s.CreateNode(ctx, domain.NodeCreate{MachineID: "m_old", Name: "Live", Protocol: "socks", ServerPort: 1080, Config: json.RawMessage(`{"udp":true}`), Endpoints: []domain.NodeEndpoint{{Name: "Old", Host: "old.example.test", Port: 1080}}})
	if err != nil {
		t.Fatal(err)
	}
	node, err = s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.AgentNodeSpec(ctx, "m_old", node.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	var beforeEvents int
	pool.QueryRow(ctx, `SELECT count(*) FROM control_changes`).Scan(&beforeEvents)
	newMachine, newName, newPort, newConfig := "m_new", "Pending", 2080, json.RawMessage(`{"udp":false}`)
	endpoints := []domain.NodeEndpoint{{Name: "New", Host: "new.example.test", Port: 2080}}
	draft, err := s.UpdateNode(ctx, node.ID, domain.NodeUpdate{MachineID: &newMachine, Name: &newName, ServerPort: &newPort, Config: &newConfig, Endpoints: &endpoints})
	if err != nil {
		t.Fatal(err)
	}
	if !draft.HasDraft || draft.Status != "published" || draft.Name != newName || draft.CurrentRevision != before.Revision {
		t.Fatalf("draft state=%+v", draft)
	}
	oldNodes, err := s.AgentNodes(ctx, "m_old")
	if err != nil || len(oldNodes) != 1 {
		t.Fatalf("old nodes=%+v %v", oldNodes, err)
	}
	newNodes, err := s.AgentNodes(ctx, "m_new")
	if err != nil || len(newNodes) != 0 {
		t.Fatalf("new nodes=%+v %v", newNodes, err)
	}
	after, err := s.AgentNodeSpec(ctx, "m_old", node.AgentID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("running spec changed before=%+v after=%+v err=%v", before, after, err)
	}
	var host string
	if err := pool.QueryRow(ctx, `SELECT host FROM node_endpoints WHERE node_id=$1`, node.ID).Scan(&host); err != nil || host != "old.example.test" {
		t.Fatalf("live endpoint=%s err=%v", host, err)
	}
	var events int
	pool.QueryRow(ctx, `SELECT count(*) FROM control_changes`).Scan(&events)
	if events != beforeEvents {
		t.Fatalf("saving draft emitted events: %d -> %d", beforeEvents, events)
	}
	// A resource claiming the draft port after it was saved must not break the old service.
	other, err := s.CreateNode(ctx, domain.NodeCreate{MachineID: "m_new", Name: "Other", Protocol: "socks", ServerPort: 2080})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PublishNode(ctx, node.ID, ""); !errors.Is(err, ErrNodePortInUse) {
		t.Fatalf("publish error=%v", err)
	}
	after, err = s.AgentNodeSpec(ctx, "m_old", node.AgentID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed publish changed live configuration", err)
	}
	stillDraft, err := s.GetNode(ctx, node.ID)
	if err != nil || !stillDraft.HasDraft {
		t.Fatal("failed publish lost draft", err)
	}
	if err := s.ArchiveNode(ctx, other.ID); err != nil {
		t.Fatal(err)
	}
	published, err := s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if published.HasDraft || published.CurrentRevision != before.Revision+1 {
		t.Fatalf("publish state=%+v", published)
	}
	if nodes, _ := s.AgentNodes(ctx, "m_old"); len(nodes) != 0 {
		t.Fatal("old machine still owns moved node")
	}
	current, err := s.AgentNodeSpec(ctx, "m_new", node.AgentID)
	if err != nil || current.ServerPort != 2080 {
		t.Fatalf("new config=%+v %v", current, err)
	}
}

func TestNodeHistoryRestoresResolvedDependenciesAsDraftPostgres(t *testing.T) {
	ctx, pool := resourcePostgresPool(t)
	s := New(pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO machines(id,name) VALUES('m_history','History')`)
	exec(`INSERT INTO outbounds(id,name,tag,protocol,settings) VALUES('out_history','Proxy','proxy','socks','{"servers":[{"address":"old.example.test","port":1080}]}')`)
	exec(`INSERT INTO route_policies(id,name,status,current_revision,default_outbound_tag) VALUES('r_history','History','published',1,'proxy')`)
	exec(`INSERT INTO route_policy_revisions(id,route_policy_id,revision,rules) VALUES('rr_history','r_history',1,'[]')`)
	policy := "r_history"
	node, err := s.CreateNode(ctx, domain.NodeCreate{MachineID: "m_history", Name: "History", Protocol: "socks", ServerPort: 1080, RoutePolicyID: &policy})
	if err != nil {
		t.Fatal(err)
	}
	node, err = s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	original, err := s.AgentNodeSpec(ctx, "m_history", node.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	settings := json.RawMessage(`{"servers":[{"address":"new.example.test","port":1080}]}`)
	if _, err := s.UpdateOutbound(ctx, "out_history", domain.OutboundUpdate{Settings: &settings}); err != nil {
		t.Fatal(err)
	}
	changed, err := s.AgentNodeSpec(ctx, "m_history", node.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(original.Settings, changed.Settings) {
		t.Fatal("dependency change did not create new spec")
	}
	revisions, err := s.ListNodeRevisions(ctx, node.ID)
	if err != nil || len(revisions) != 2 || !revisions[0].Restorable || !revisions[1].Restorable {
		t.Fatalf("history=%+v err=%v", revisions, err)
	}
	draft, err := s.RestoreNodeRevision(ctx, node.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !draft.HasDraft || draft.RestoredFromRevision != 1 || draft.RoutePolicyID != nil {
		t.Fatalf("restored draft=%+v", draft)
	}
	live, err := s.AgentNodeSpec(ctx, "m_history", node.AgentID)
	if err != nil || !reflect.DeepEqual(live, changed) {
		t.Fatal("restoration affected live service", err)
	}
	published, err := s.PublishNode(ctx, node.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := s.AgentNodeSpec(ctx, "m_history", node.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if published.CurrentRevision != 3 || !reflect.DeepEqual(original.Settings, restored.Settings) {
		t.Fatalf("rollback config differs: %s vs %s", original.Settings, restored.Settings)
	}
	exec(`INSERT INTO node_revisions(id,node_id,revision,config) VALUES('legacy',$1,0,'{}')`, node.ID)
	if _, err := s.RestoreNodeRevision(ctx, node.ID, 0); !errors.Is(err, ErrIncompleteNodeRevision) {
		t.Fatalf("incomplete history restore=%v", err)
	}
}

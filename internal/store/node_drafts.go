package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

type nodeQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type nodeDraft struct {
	domain.NodeCreate
	RestoredFromRevision int `json:"restored_from_revision,omitempty"`
}

type nodeSnapshot struct {
	Node domain.NodeCreate    `json:"node"`
	Spec domain.AgentNodeSpec `json:"spec"`
}

type NodeRevision struct {
	Revision    int             `json:"revision"`
	CreatedAt   time.Time       `json:"created_at"`
	PublishedBy *string         `json:"published_by"`
	Restorable  bool            `json:"restorable"`
	Config      json.RawMessage `json:"config"`
	Snapshot    json.RawMessage `json:"snapshot,omitempty"`
}

func nodeDocument(item domain.Node) domain.NodeCreate {
	return domain.NodeCreate{MachineID: item.MachineID, RoutePolicyID: item.RoutePolicyID,
		AdminRoutePolicyID: item.AdminRoutePolicyID, MemberRoutePolicyID: item.MemberRoutePolicyID,
		Name: item.Name, Protocol: item.Protocol, ListenIP: item.ListenIP, ServerPort: item.ServerPort,
		KernelType: item.KernelType, Config: item.Config, Endpoints: item.Endpoints}
}

func overlayNodeDocument(item *domain.Node, draft domain.NodeCreate) {
	item.MachineID, item.RoutePolicyID = draft.MachineID, draft.RoutePolicyID
	item.AdminRoutePolicyID, item.MemberRoutePolicyID = draft.AdminRoutePolicyID, draft.MemberRoutePolicyID
	item.Name, item.Protocol, item.ListenIP = draft.Name, draft.Protocol, draft.ListenIP
	item.ServerPort, item.KernelType, item.Config = draft.ServerPort, draft.KernelType, draft.Config
	item.Endpoints = draft.Endpoints
}

func loadNodeDraft(ctx context.Context, q nodeQueryer, item *domain.Node) error {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT document FROM node_drafts WHERE node_id=$1`, item.ID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var draft nodeDraft
	if err := json.Unmarshal(raw, &draft); err != nil {
		return err
	}
	previousMachine := item.MachineID
	overlayNodeDocument(item, draft.NodeCreate)
	item.HasDraft, item.RestoredFromRevision = true, draft.RestoredFromRevision
	if previousMachine != item.MachineID {
		if err := q.QueryRow(ctx, `SELECT name FROM machines WHERE id=$1`, item.MachineID).Scan(&item.MachineName); err != nil {
			return err
		}
	}
	return nil
}

func saveNodeDraft(ctx context.Context, tx pgx.Tx, item domain.Node) error {
	raw, err := json.Marshal(nodeDraft{NodeCreate: nodeDocument(item), RestoredFromRevision: item.RestoredFromRevision})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO node_drafts(node_id,document) VALUES($1,$2)
		ON CONFLICT(node_id) DO UPDATE SET document=EXCLUDED.document,updated_at=now()`, item.ID, raw)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE nodes SET updated_at=now() WHERE id=$1`, item.ID)
	return err
}

func runtimeNode(ctx context.Context, q nodeQueryer, id string) (domain.Node, error) {
	var item domain.Node
	err := q.QueryRow(ctx, `SELECT n.id,n.agent_id,n.machine_id,m.name,n.route_policy_id,n.admin_route_policy_id,n.member_route_policy_id,
		n.name,n.protocol,n.listen_ip,n.server_port,n.kernel_type,n.config,n.status,n.current_revision,n.applied_revision,
		n.received_revision,n.attempted_revision,n.apply_status,n.apply_stage,
		n.last_report_at,n.last_error,n.created_at,n.updated_at
		FROM nodes n JOIN machines m ON m.id=n.machine_id WHERE n.id=$1 AND n.status<>'archived'`, id).Scan(
		&item.ID, &item.AgentID, &item.MachineID, &item.MachineName, &item.RoutePolicyID, &item.AdminRoutePolicyID, &item.MemberRoutePolicyID,
		&item.Name, &item.Protocol, &item.ListenIP, &item.ServerPort, &item.KernelType, &item.Config, &item.Status, &item.CurrentRevision, &item.AppliedRevision,
		&item.ReceivedRevision, &item.AttemptedRevision, &item.ApplyStatus, &item.ApplyStage,
		&item.LastReport, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	item.Endpoints, err = queryNodeEndpoints(ctx, q, id)
	return item, err
}

func captureNodeSnapshot(ctx context.Context, tx pgx.Tx, nodeID, adminID string) error {
	item, err := runtimeNode(ctx, tx, nodeID)
	if err != nil {
		return err
	}
	if item.CurrentRevision == 0 || item.Status != "published" {
		return nil
	}
	spec, err := compileAgentNodeSpec(ctx, tx, item.MachineID, item.AgentID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(nodeSnapshot{Node: nodeDocument(item), Spec: spec})
	if err != nil {
		return err
	}
	var publisher any
	if adminID != "" {
		publisher = adminID
	}
	_, err = tx.Exec(ctx, `INSERT INTO node_revisions(id,node_id,revision,config,published_by,snapshot)
		VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(node_id,revision) DO UPDATE SET snapshot=EXCLUDED.snapshot
		WHERE node_revisions.snapshot IS NULL`, domain.MustID("nrv"), item.ID, item.CurrentRevision, item.Config, publisher, raw)
	return err
}

func capturePublishedSnapshots(ctx context.Context, tx pgx.Tx, policyID string) error {
	rows, err := tx.Query(ctx, `SELECT id FROM nodes WHERE status='published'
		AND ($1='' OR $1 IN (route_policy_id,admin_route_policy_id,member_route_policy_id)) ORDER BY id`, policyID)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := captureNodeSnapshot(ctx, tx, id, ""); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListNodeRevisions(ctx context.Context, id string) ([]NodeRevision, error) {
	if _, err := s.GetNode(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT revision,created_at,published_by,snapshot IS NOT NULL,config,snapshot
		FROM node_revisions WHERE node_id=$1 ORDER BY revision DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]NodeRevision, 0)
	for rows.Next() {
		var item NodeRevision
		if err := rows.Scan(&item.Revision, &item.CreatedAt, &item.PublishedBy, &item.Restorable, &item.Config, &item.Snapshot); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Restore produces an editable draft. Resolved routing dependencies are copied
// into this node's settings, avoiding mutation of shared routes or outbounds.
func (s *Store) RestoreNodeRevision(ctx context.Context, id string, revision int) (domain.Node, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Node{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.Node{}, err
	}
	var locked string
	if err := tx.QueryRow(ctx, `SELECT id FROM nodes WHERE id=$1 AND status<>'archived' FOR UPDATE`, id).Scan(&locked); errors.Is(err, pgx.ErrNoRows) {
		return domain.Node{}, ErrNotFound
	} else if err != nil {
		return domain.Node{}, err
	}
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT snapshot FROM node_revisions WHERE node_id=$1 AND revision=$2`, id, revision).Scan(&raw); errors.Is(err, pgx.ErrNoRows) {
		return domain.Node{}, ErrNotFound
	} else if err != nil {
		return domain.Node{}, err
	}
	if len(raw) == 0 {
		return domain.Node{}, ErrIncompleteNodeRevision
	}
	var snapshot nodeSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return domain.Node{}, err
	}
	item, err := runtimeNode(ctx, tx, id)
	if err != nil {
		return domain.Node{}, err
	}
	snapshot.Node.Config = snapshot.Spec.Settings
	snapshot.Node.RoutePolicyID = nil
	snapshot.Node.AdminRoutePolicyID = nil
	snapshot.Node.MemberRoutePolicyID = nil
	overlayNodeDocument(&item, snapshot.Node)
	item.RestoredFromRevision = revision
	if err := s.validateNodeBindingsTx(ctx, tx, item.MachineID, nil, nil, nil, item.KernelType); err != nil {
		return domain.Node{}, err
	}
	if err := saveNodeDraft(ctx, tx, item); err != nil {
		return domain.Node{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Node{}, err
	}
	return s.GetNode(ctx, id)
}

func hasNodeConfiguration(input domain.NodeUpdate) bool {
	return input.MachineID != nil || input.RoutePolicyID != nil || input.AdminRoutePolicyID != nil || input.MemberRoutePolicyID != nil ||
		input.Name != nil || input.Protocol != nil || input.ListenIP != nil || input.ServerPort != nil || input.KernelType != nil || input.Config != nil || input.Endpoints != nil
}

func validateDraftPort(ctx context.Context, tx pgx.Tx, item domain.Node) error {
	var occupied bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM nodes WHERE machine_id=$1 AND server_port=$2 AND id<>$3 AND status<>'archived')`, item.MachineID, item.ServerPort, item.ID).Scan(&occupied); err != nil {
		return err
	}
	if occupied {
		return ErrNodePortInUse
	}
	if item.ServerPort < 1 || item.ServerPort > 65535 || strings.TrimSpace(item.Name) == "" {
		return ErrConflict
	}
	return nil
}

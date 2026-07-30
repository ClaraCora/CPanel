package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) AuthenticateAgent(ctx context.Context, tokenHash []byte) (domain.AgentMachine, error) {
	var machine domain.AgentMachine
	err := s.pool.QueryRow(ctx, `
		SELECT m.id,m.name,m.status,m.kernel_type,c.id,m.capabilities
		FROM machine_credentials c JOIN machines m ON m.id=c.machine_id
		WHERE c.token_hash=$1 AND c.revoked_at IS NULL
		  AND (c.expires_at IS NULL OR c.expires_at > now())
		  AND m.status NOT IN ('disabled','archived')`, tokenHash,
	).Scan(&machine.ID, &machine.Name, &machine.Status, &machine.KernelType, &machine.TokenID, &machine.Capabilities)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentMachine{}, ErrNotFound
	}
	if err != nil {
		return domain.AgentMachine{}, err
	}
	_, _ = s.pool.Exec(ctx, "UPDATE machine_credentials SET last_used_at=now() WHERE id=$1", machine.TokenID)
	return machine, nil
}

func (s *Store) AuthenticateSharedAgent(ctx context.Context, machineID string, tokenHash []byte) (domain.AgentMachine, error) {
	var machine domain.AgentMachine
	err := s.pool.QueryRow(ctx, `
		SELECT m.id,m.name,m.status,m.kernel_type,'shared',m.capabilities
		FROM agent_shared_credentials c CROSS JOIN machines m
		WHERE c.token_hash=$1 AND m.id=$2 AND m.status NOT IN ('disabled','archived')`,
		tokenHash, machineID,
	).Scan(&machine.ID, &machine.Name, &machine.Status, &machine.KernelType, &machine.TokenID, &machine.Capabilities)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentMachine{}, ErrNotFound
	}
	return machine, err
}

func (s *Store) AgentNodes(ctx context.Context, machineID string) ([]domain.AgentNode, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT agent_id,protocol,name FROM nodes
		WHERE machine_id=$1 AND status='published' ORDER BY agent_id`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AgentNode, 0)
	for rows.Next() {
		var item domain.AgentNode
		if err := rows.Scan(&item.AgentID, &item.Type, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AgentNodeSpec(ctx context.Context, machineID string, agentNodeID int64) (domain.AgentNodeSpec, error) {
	var item domain.AgentNodeSpec
	err := s.pool.QueryRow(ctx, `SELECT agent_id,current_revision,protocol,listen_ip,server_port,kernel_type,config FROM nodes
		WHERE machine_id=$1 AND agent_id=$2 AND status='published'`, machineID, agentNodeID).Scan(
		&item.NodeID, &item.Revision, &item.Protocol, &item.ListenIP, &item.ServerPort, &item.KernelType, &item.Settings,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentNodeSpec{}, ErrNotFound
	}
	return item, err
}

func (s *Store) AgentNodeUsers(ctx context.Context, machineID string, agentNodeID int64) ([]domain.AgentUser, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.agent_id,u.uuid,
		       COALESCE(u.speed_limit_override_mbps,p.speed_limit_mbps,0),
		       COALESCE(u.device_limit_override,p.device_limit,0)
		FROM nodes n
		JOIN access_group_nodes gn ON gn.node_id=n.id
		JOIN users u ON true
		LEFT JOIN plans p ON p.id=u.plan_id
		WHERE n.machine_id=$1 AND n.agent_id=$2 AND n.status='published'
		  AND gn.access_group_id=COALESCE(u.access_group_override_id,p.access_group_id)
		  AND u.status='active' AND (u.expires_at IS NULL OR u.expires_at > now())
		ORDER BY u.agent_id`, machineID, agentNodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AgentUser, 0)
	for rows.Next() {
		var item domain.AgentUser
		if err := rows.Scan(&item.ID, &item.UUID, &item.SpeedLimit, &item.DeviceLimit); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AgentChanges(ctx context.Context, machineID string, after int64, limit int) ([]domain.ControlChange, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.cursor,n.agent_id,c.event_type,c.revision,c.payload,c.created_at
		FROM control_changes c LEFT JOIN nodes n ON n.id=c.node_id
		WHERE c.machine_id=$1 AND c.cursor>$2 ORDER BY c.cursor LIMIT $3`, machineID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ControlChange, 0)
	for rows.Next() {
		var item domain.ControlChange
		if err := rows.Scan(&item.Cursor, &item.NodeID, &item.EventType, &item.Revision, &item.Payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) LatestAgentCursor(ctx context.Context, machineID string) (int64, error) {
	var cursor int64
	err := s.pool.QueryRow(ctx, "SELECT COALESCE(max(cursor),0) FROM control_changes WHERE machine_id=$1", machineID).Scan(&cursor)
	return cursor, err
}

func (s *Store) RecordMachineHeartbeat(ctx context.Context, machine domain.AgentMachine, version, kernel string, capabilities, metrics json.RawMessage) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if len(capabilities) == 0 {
		capabilities = json.RawMessage(`{}`)
	}
	if len(metrics) == 0 {
		metrics = json.RawMessage(`{}`)
	}
	_, err = tx.Exec(ctx, `UPDATE machines SET status='online',agent_version=$2,kernel_type=$3,
		capabilities=$4,last_heartbeat_at=now(),updated_at=now(),
		agent_upgrade_task_id=CASE WHEN agent_upgrade_dispatched_at IS NOT NULL THEN NULL ELSE agent_upgrade_task_id END,
		agent_upgrade_requested_at=CASE WHEN agent_upgrade_dispatched_at IS NOT NULL THEN NULL ELSE agent_upgrade_requested_at END,
		agent_upgrade_dispatched_at=CASE WHEN agent_upgrade_dispatched_at IS NOT NULL THEN NULL ELSE agent_upgrade_dispatched_at END
		WHERE id=$1`,
		machine.ID, version, kernel, capabilities)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO machine_metrics(machine_id,sampled_at,metrics) VALUES($1,$2,$3)
		ON CONFLICT(machine_id) DO UPDATE SET sampled_at=EXCLUDED.sampled_at,metrics=EXCLUDED.metrics`,
		machine.ID, time.Now().UTC(), metrics)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ClaimMachineAgentUpgrade(ctx context.Context, machineID string) (domain.AgentCommand, bool, error) {
	var command domain.AgentCommand
	err := s.pool.QueryRow(ctx, `UPDATE machines SET agent_upgrade_dispatched_at=now(),updated_at=now()
		WHERE id=$1 AND agent_upgrade_task_id IS NOT NULL AND agent_upgrade_dispatched_at IS NULL
		RETURNING agent_upgrade_task_id`, machineID).Scan(&command.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentCommand{}, false, nil
	}
	if err != nil {
		return domain.AgentCommand{}, false, err
	}
	command.Type = "agent.upgrade"
	return command, true, nil
}

func (s *Store) RecordTelemetryBatch(ctx context.Context, machineID, idempotencyKey string, payload json.RawMessage) (bool, error) {
	batch, err := decodeTelemetryBatch(payload)
	if err != nil {
		return false, err
	}
	trafficLocation := loadTrafficLocation(s.SettingString(ctx, "site", "timezone", "Asia/Shanghai"))
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO telemetry_batches(machine_id,idempotency_key)
		VALUES($1,$2) ON CONFLICT DO NOTHING`, machineID, idempotencyKey)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	for _, event := range batch.Events {
		switch event.Type {
		case "node.telemetry":
			if err := recordNodeTelemetry(ctx, tx, machineID, event, trafficLocation); err != nil {
				return false, err
			}
		case "node.devices":
			if err := recordNodeDevices(ctx, tx, machineID, event); err != nil {
				return false, err
			}
		}
	}
	return true, tx.Commit(ctx)
}

type telemetryBatch struct {
	Events []telemetryEvent `json:"events"`
}

type telemetryEvent struct {
	Type       string          `json:"type"`
	NodeID     int64           `json:"node_id"`
	OccurredAt string          `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

type nodeTelemetryData struct {
	Revision int64               `json:"revision"`
	Traffic  map[string][2]int64 `json:"traffic"`
}

type nodeDevicesData struct {
	Devices map[string][]string `json:"devices"`
}

func decodeTelemetryBatch(payload json.RawMessage) (telemetryBatch, error) {
	var batch telemetryBatch
	if err := json.Unmarshal(payload, &batch); err != nil {
		return telemetryBatch{}, fmt.Errorf("decode telemetry batch: %w", err)
	}
	if batch.Events == nil {
		return telemetryBatch{}, fmt.Errorf("telemetry events are required")
	}
	for index, event := range batch.Events {
		if event.Type == "" {
			return telemetryBatch{}, fmt.Errorf("telemetry event %d type is required", index)
		}
		if event.NodeID < 0 {
			return telemetryBatch{}, fmt.Errorf("telemetry event %d node_id is invalid", index)
		}
	}
	return batch, nil
}

func telemetryTime(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC()
	}
	return time.Now().UTC()
}

func loadTrafficLocation(name string) *time.Location {
	if location, err := time.LoadLocation(strings.TrimSpace(name)); err == nil {
		return location
	}
	if location, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return location
	}
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

func telemetryDay(sampledAt time.Time, location *time.Location) string {
	if location == nil {
		location = loadTrafficLocation("")
	}
	return sampledAt.In(location).Format(time.DateOnly)
}

func recordNodeTelemetry(ctx context.Context, tx pgx.Tx, machineID string, event telemetryEvent, trafficLocation *time.Location) error {
	if event.NodeID <= 0 {
		return fmt.Errorf("node telemetry node_id is required")
	}
	var data nodeTelemetryData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("decode node telemetry: %w", err)
	}
	sampledAt := telemetryTime(event.OccurredAt)
	var nodeID string
	err := tx.QueryRow(ctx, `
		UPDATE nodes SET applied_revision=CASE WHEN $3 > 0 THEN LEAST(current_revision,GREATEST(applied_revision,$3)) ELSE applied_revision END,
		       last_report_at=$4,updated_at=now()
		WHERE machine_id=$1 AND agent_id=$2 AND status='published'
		RETURNING id`, machineID, event.NodeID, data.Revision, sampledAt).Scan(&nodeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO node_metrics(node_id,sampled_at,metrics) VALUES($1,$2,$3)
		ON CONFLICT(node_id) DO UPDATE SET sampled_at=EXCLUDED.sampled_at,metrics=EXCLUDED.metrics
		WHERE EXCLUDED.sampled_at >= node_metrics.sampled_at`, nodeID, sampledAt, event.Data); err != nil {
		return err
	}
	for userAgentID, traffic := range data.Traffic {
		userID, err := strconv.ParseInt(userAgentID, 10, 64)
		if err != nil || userID <= 0 || traffic[0] < 0 || traffic[1] < 0 || (traffic[0] == 0 && traffic[1] == 0) {
			continue
		}
		_, err = tx.Exec(ctx, `
			WITH entitled AS (
			  SELECT u.id AS user_id,n.id AS node_id
			FROM nodes n
			JOIN access_group_nodes gn ON gn.node_id=n.id
			JOIN users u ON u.agent_id=$3
			LEFT JOIN plans p ON p.id=u.plan_id
			WHERE n.id=$1 AND n.machine_id=$2
			  AND gn.access_group_id=COALESCE(u.access_group_override_id,p.access_group_id)
			), recorded AS (
			  INSERT INTO traffic_daily(day,user_id,node_id,upload_bytes,download_bytes)
			  SELECT $4::date,user_id,node_id,$5,$6 FROM entitled
			ON CONFLICT(day,user_id,node_id) DO UPDATE SET
			  upload_bytes=traffic_daily.upload_bytes+EXCLUDED.upload_bytes,
			  download_bytes=traffic_daily.download_bytes+EXCLUDED.download_bytes
			  RETURNING user_id
			)
			UPDATE users SET traffic_used_bytes=traffic_used_bytes+$5+$6
			WHERE id IN (SELECT user_id FROM recorded)`,
			nodeID, machineID, userID, telemetryDay(sampledAt, trafficLocation), traffic[0], traffic[1])
		if err != nil {
			return err
		}
	}
	return nil
}

func recordNodeDevices(ctx context.Context, tx pgx.Tx, machineID string, event telemetryEvent) error {
	if event.NodeID <= 0 {
		return fmt.Errorf("node devices node_id is required")
	}
	var data nodeDevicesData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return fmt.Errorf("decode node devices: %w", err)
	}
	sampledAt := telemetryTime(event.OccurredAt)
	for userAgentID, addresses := range data.Devices {
		userID, err := strconv.ParseInt(userAgentID, 10, 64)
		if err != nil || userID <= 0 {
			continue
		}
		for _, address := range addresses {
			if net.ParseIP(address) == nil {
				continue
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO user_devices(id,user_id,node_id,ip_address,first_seen_at,last_seen_at,online)
				SELECT $6,u.id,n.id,$4::inet,$5,$5,true
				FROM nodes n
				JOIN access_group_nodes gn ON gn.node_id=n.id
				JOIN users u ON u.agent_id=$3
				LEFT JOIN plans p ON p.id=u.plan_id
				WHERE n.machine_id=$1 AND n.agent_id=$2
				  AND gn.access_group_id=COALESCE(u.access_group_override_id,p.access_group_id)
				ON CONFLICT(user_id,node_id,ip_address) DO UPDATE SET last_seen_at=EXCLUDED.last_seen_at,online=true`,
				machineID, event.NodeID, userID, address, sampledAt, domain.MustID("dev"))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

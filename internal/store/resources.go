package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Store) Overview(ctx context.Context, rankingPeriod string) (domain.Overview, error) {
	var overview domain.Overview
	currentDay := time.Now().In(loadTrafficLocation(s.SettingString(ctx, "site", "timezone", "Asia/Shanghai")))
	startOffset, endOffset := rankingDateOffsets(rankingPeriod, currentDay)
	if _, err := s.ReconcileMachinePresence(ctx); err != nil {
		return overview, err
	}
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM machines WHERE status <> 'archived'),
			(SELECT count(*) FROM machines WHERE status = 'online'),
			(SELECT count(*) FROM machines WHERE status = 'offline'),
			(SELECT count(*) FROM nodes WHERE status <> 'archived'),
			(SELECT count(*) FROM nodes WHERE status = 'published'),
			(SELECT count(*) FROM users WHERE role='admin' AND status='active'),
			(SELECT count(*) FROM users WHERE role='user' AND status='active'),
			(SELECT count(*) FROM users WHERE role='friend' AND status='active'),
			COALESCE((SELECT sum(upload_bytes) FROM traffic_daily WHERE day=$1::date), 0) +
			  COALESCE((SELECT sum(upload_bytes) FROM imported_user_traffic_daily WHERE day=$1::date), 0),
			COALESCE((SELECT sum(download_bytes) FROM traffic_daily WHERE day=$1::date), 0) +
			  COALESCE((SELECT sum(download_bytes) FROM imported_user_traffic_daily WHERE day=$1::date), 0)
	`, currentDay.Format(time.DateOnly)).Scan(
		&overview.MachinesTotal, &overview.MachinesOnline, &overview.MachinesOffline,
		&overview.NodesTotal, &overview.NodesPublished, &overview.AdminsActive, &overview.UsersActive,
		&overview.FriendsActive, &overview.TrafficTodayUpload, &overview.TrafficTodayDownload,
	)
	if err != nil {
		return overview, err
	}
	overview.TrafficToday = overview.TrafficTodayUpload + overview.TrafficTodayDownload
	overview.NodeTrafficRanking, err = s.trafficRanking(ctx, `WITH combined AS (
		SELECT node_id,upload_bytes,download_bytes FROM traffic_daily
		WHERE day >= $1::date + $2::int AND day < $1::date + $3::int
		UNION ALL SELECT node_id,upload_bytes,download_bytes FROM imported_node_traffic_daily
		WHERE day >= $1::date + $2::int AND day < $1::date + $3::int)
		SELECT n.id,n.name,sum(t.upload_bytes),sum(t.download_bytes),sum(t.upload_bytes+t.download_bytes)
		FROM combined t JOIN nodes n ON n.id=t.node_id GROUP BY n.id,n.name
		ORDER BY sum(t.upload_bytes+t.download_bytes) DESC,n.name LIMIT 10`, currentDay.Format(time.DateOnly), startOffset, endOffset)
	if err != nil {
		return overview, err
	}
	overview.UserTrafficRanking, err = s.trafficRanking(ctx, `WITH combined AS (
		SELECT user_id,upload_bytes,download_bytes FROM traffic_daily
		WHERE day >= $1::date + $2::int AND day < $1::date + $3::int
		UNION ALL SELECT user_id,upload_bytes,download_bytes FROM imported_user_traffic_daily
		WHERE day >= $1::date + $2::int AND day < $1::date + $3::int)
		SELECT u.id,u.name,sum(t.upload_bytes),sum(t.download_bytes),sum(t.upload_bytes+t.download_bytes)
		FROM combined t JOIN users u ON u.id=t.user_id GROUP BY u.id,u.name
		ORDER BY sum(t.upload_bytes+t.download_bytes) DESC,u.name LIMIT 10`, currentDay.Format(time.DateOnly), startOffset, endOffset)
	return overview, err
}

func (s *Store) TrafficSummary(ctx context.Context, period string) (domain.TrafficSummary, error) {
	var summary domain.TrafficSummary
	currentDay := time.Now().In(loadTrafficLocation(s.SettingString(ctx, "site", "timezone", "Asia/Shanghai")))
	startOffset, endOffset := rankingDateOffsets(period, currentDay)
	err := s.pool.QueryRow(ctx, `WITH combined AS (
		SELECT upload_bytes,download_bytes FROM traffic_daily
		WHERE day >= $1::date + $2::int AND day < $1::date + $3::int
		UNION ALL SELECT upload_bytes,download_bytes FROM imported_user_traffic_daily
		WHERE day >= $1::date + $2::int AND day < $1::date + $3::int)
		SELECT COALESCE(sum(upload_bytes),0),COALESCE(sum(download_bytes),0) FROM combined`,
		currentDay.Format(time.DateOnly), startOffset, endOffset).Scan(&summary.UploadBytes, &summary.DownloadBytes)
	summary.TotalBytes = summary.UploadBytes + summary.DownloadBytes
	return summary, err
}

func rankingDateOffsets(period string, currentDay time.Time) (int, int) {
	switch period {
	case "yesterday":
		return -1, 0
	case "7d":
		return -6, 1
	case "month":
		return -(currentDay.Day() - 1), 1
	default:
		return 0, 1
	}
}

func (s *Store) trafficRanking(ctx context.Context, query string, args ...any) ([]domain.TrafficRank, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.TrafficRank, 0)
	for rows.Next() {
		var item domain.TrafficRank
		if err := rows.Scan(&item.ID, &item.Name, &item.UploadBytes, &item.DownloadBytes, &item.TotalBytes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListMachines(ctx context.Context) ([]domain.Machine, error) {
	threshold := s.machineOfflineThreshold(ctx)
	if _, err := s.reconcileMachinePresence(ctx, threshold); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.name, m.region, m.host, m.labels, m.notes, m.status,
		       m.agent_version, m.kernel_type, m.capabilities, m.last_heartbeat_at,
		       COALESCE(mm.metrics,'{}'::jsonb),mm.sampled_at,
		       COALESCE(m.agent_upgrade_task_id,''),m.agent_upgrade_requested_at,m.agent_upgrade_dispatched_at,
		       COALESCE(m.agent_upgrade_target_version,''),COALESCE(m.agent_upgrade_status,''),
		       m.agent_upgrade_acknowledged_at,m.agent_upgrade_completed_at,m.agent_upgrade_failed_at,
		       COALESCE(m.agent_upgrade_error,''),
		       m.agent_protocol,m.agent_v2_last_seen_at,
		       (SELECT count(*) FROM nodes n WHERE n.machine_id=m.id AND n.status <> 'archived'),
		       m.created_at, m.updated_at
		FROM machines m LEFT JOIN machine_metrics mm ON mm.machine_id=m.id
		WHERE m.status <> 'archived'
		ORDER BY m.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Machine, 0)
	now := time.Now()
	for rows.Next() {
		var item domain.Machine
		if err := rows.Scan(&item.ID, &item.Name, &item.Region, &item.Host, &item.Labels, &item.Notes,
			&item.Status, &item.AgentVersion, &item.KernelType, &item.Capabilities, &item.LastHeartbeat,
			&item.Metrics, &item.MetricsSampledAt, &item.AgentUpgradeTaskID, &item.AgentUpgradeRequestedAt,
			&item.AgentUpgradeDispatchedAt, &item.AgentUpgradeTargetVersion, &item.AgentUpgradeStatus,
			&item.AgentUpgradeAcknowledgedAt, &item.AgentUpgradeCompletedAt, &item.AgentUpgradeFailedAt,
			&item.AgentUpgradeError, &item.AgentProtocol, &item.AgentV2LastSeenAt,
			&item.NodeCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Status = effectiveMachineStatus(item.Status, item.LastHeartbeat, now, threshold)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) machineOfflineThreshold(ctx context.Context) time.Duration {
	seconds := s.SettingInt(ctx, "agent", "offline_threshold_seconds", 180, 10)
	return time.Duration(seconds) * time.Second
}

func (s *Store) ReconcileMachinePresence(ctx context.Context) (int64, error) {
	return s.reconcileMachinePresence(ctx, s.machineOfflineThreshold(ctx))
}

func (s *Store) reconcileMachinePresence(ctx context.Context, threshold time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE machines SET status='offline',updated_at=now()
		WHERE status='online'
		  AND (last_heartbeat_at IS NULL OR last_heartbeat_at < now() - ($1::bigint * interval '1 second'))`,
		int64(threshold/time.Second))
	if err != nil {
		return 0, err
	}
	// Do not leave a dispatched upgrade permanently stuck when the Agent
	// disappeared during the detached installer. A terminal timeout remains
	// visible for audit purposes and can be requested again safely.
	if _, err := s.pool.Exec(ctx, `UPDATE machines SET
		agent_upgrade_status='timed_out',agent_upgrade_failed_at=COALESCE(agent_upgrade_failed_at,now()),
		agent_upgrade_error=CASE WHEN agent_upgrade_error='' THEN 'Agent upgrade timed out' ELSE agent_upgrade_error END,
		updated_at=now()
		WHERE agent_upgrade_task_id IS NOT NULL
		  AND agent_upgrade_status IN ('dispatched','acknowledged')
		  AND agent_upgrade_dispatched_at < now() - interval '15 minutes'`); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func effectiveMachineStatus(status string, lastHeartbeat *time.Time, now time.Time, threshold time.Duration) string {
	if status != "online" {
		return status
	}
	if lastHeartbeat == nil || lastHeartbeat.Before(now.Add(-threshold)) {
		return "offline"
	}
	return status
}

func (s *Store) RequestMachineAgentUpgrade(ctx context.Context, machineID, latestVersion string) (domain.AgentUpgradeTask, error) {
	var connected, pending bool
	var currentVersion string
	threshold := s.machineOfflineThreshold(ctx)
	err := s.pool.QueryRow(ctx, `SELECT status='online' AND last_heartbeat_at IS NOT NULL
		AND last_heartbeat_at >= now() - ($2::bigint * interval '1 second'),
		agent_version,agent_upgrade_task_id IS NOT NULL AND agent_upgrade_status IN ('queued','dispatched','acknowledged')
		FROM machines WHERE id=$1 AND status NOT IN ('archived','disabled')`, machineID, int64(threshold/time.Second)).Scan(&connected, &currentVersion, &pending)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentUpgradeTask{}, ErrNotFound
	}
	if err != nil {
		return domain.AgentUpgradeTask{}, err
	}
	if !connected {
		return domain.AgentUpgradeTask{}, ErrAgentNotConnected
	}
	if pending {
		return domain.AgentUpgradeTask{}, ErrAgentUpgradePending
	}
	if latestVersion != "" && sameAgentVersion(currentVersion, latestVersion) {
		return domain.AgentUpgradeTask{}, ErrAgentAlreadyLatest
	}
	task := domain.AgentUpgradeTask{ID: domain.MustID("upg"), MachineID: machineID, TargetVersion: strings.TrimSpace(latestVersion), Status: "queued"}
	err = s.pool.QueryRow(ctx, `UPDATE machines SET agent_upgrade_task_id=$2,
		agent_upgrade_requested_at=now(),agent_upgrade_dispatched_at=NULL,
		agent_upgrade_target_version=$3,agent_upgrade_status='queued',
		agent_upgrade_acknowledged_at=NULL,agent_upgrade_completed_at=NULL,
		agent_upgrade_failed_at=NULL,agent_upgrade_error='',updated_at=now()
		WHERE id=$1 RETURNING agent_upgrade_requested_at`, machineID, task.ID, task.TargetVersion).Scan(&task.RequestedAt)
	return task, err
}

func (s *Store) CreateMachine(ctx context.Context, input domain.MachineCreate) (domain.Machine, error) {
	item := domain.Machine{ID: domain.MustID("mch")}
	labels := input.Labels
	if len(labels) == 0 {
		labels = json.RawMessage(`{}`)
	}
	var err error
	input.KernelType, err = normalizeMachineKernel(input.KernelType)
	if err != nil {
		return domain.Machine{}, err
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO machines(id,name,region,host,labels,notes,kernel_type)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING name,region,host,labels,notes,status,agent_version,kernel_type,capabilities,
		          last_heartbeat_at,created_at,updated_at`,
		item.ID, strings.TrimSpace(input.Name), strings.TrimSpace(input.Region), strings.TrimSpace(input.Host),
		labels, strings.TrimSpace(input.Notes), input.KernelType,
	).Scan(&item.Name, &item.Region, &item.Host, &item.Labels, &item.Notes, &item.Status,
		&item.AgentVersion, &item.KernelType, &item.Capabilities, &item.LastHeartbeat, &item.CreatedAt, &item.UpdatedAt)
	return item, mapError(err)
}

func (s *Store) ArchiveMachine(ctx context.Context, machineID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM machines WHERE id=$1 FOR UPDATE`, machineID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if status == "archived" {
		return ErrNotFound
	}
	var nodeCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM nodes WHERE machine_id=$1 AND status <> 'archived'`, machineID).Scan(&nodeCount); err != nil {
		return err
	}
	if nodeCount > 0 {
		return ErrMachineHasNodes
	}
	if _, err := tx.Exec(ctx, `UPDATE machines SET status='archived',updated_at=now() WHERE id=$1`, machineID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateMachine(ctx context.Context, machineID string, input domain.MachineUpdate) (domain.Machine, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Machine{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.Machine{}, err
	}
	var currentStatus, currentKernel string
	if err := tx.QueryRow(ctx, `SELECT status,kernel_type FROM machines WHERE id=$1 AND status <> 'archived' FOR UPDATE`, machineID).Scan(&currentStatus, &currentKernel); errors.Is(err, pgx.ErrNoRows) {
		return domain.Machine{}, ErrNotFound
	} else if err != nil {
		return domain.Machine{}, err
	}
	resultingKernel := currentKernel
	if input.KernelType != nil {
		resultingKernel, err = normalizeMachineKernel(*input.KernelType)
		if err != nil {
			return domain.Machine{}, err
		}
	}
	resultingStatus := currentStatus
	if input.Status != nil {
		resultingStatus, err = normalizeMachineStatus(*input.Status)
		if err != nil {
			return domain.Machine{}, err
		}
	}
	if resultingStatus == "disabled" && currentStatus != "disabled" {
		var nodeCount int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM nodes WHERE machine_id=$1 AND status <> 'archived'`, machineID).Scan(&nodeCount); err != nil {
			return domain.Machine{}, err
		}
		if nodeCount > 0 {
			return domain.Machine{}, ErrMachineHasNodes
		}
	}
	command, err := tx.Exec(ctx, `UPDATE machines SET
		name=COALESCE(NULLIF(BTRIM($2),''),name),region=COALESCE($3,region),host=COALESCE($4,host),
		labels=COALESCE($5,labels),notes=COALESCE($6,notes),kernel_type=$7,status=$8,updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, machineID, input.Name, input.Region, input.Host,
		input.Labels, input.Notes, resultingKernel, resultingStatus)
	if err != nil {
		return domain.Machine{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.Machine{}, ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Machine{}, err
	}
	items, err := s.ListMachines(ctx)
	if err != nil {
		return domain.Machine{}, err
	}
	for _, item := range items {
		if item.ID == machineID {
			return item, nil
		}
	}
	return domain.Machine{}, ErrNotFound
}

func (s *Store) CreateMachineCredential(ctx context.Context, machineID string, expiresAt *time.Time) (domain.MachineCredential, error) {
	plain, hash, err := auth.NewSecret("cpa_", 32)
	if err != nil {
		return domain.MachineCredential{}, err
	}
	item := domain.MachineCredential{
		ID: domain.MustID("cred"), MachineID: machineID, Token: plain,
		TokenPrefix: auth.Prefix(plain, 12), ExpiresAt: expiresAt,
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO machine_credentials(id,machine_id,token_hash,token_prefix,expires_at)
		VALUES($1,$2,$3,$4,$5) RETURNING created_at`,
		item.ID, item.MachineID, hash, item.TokenPrefix, expiresAt,
	).Scan(&item.CreatedAt)
	return item, mapError(err)
}

func (s *Store) ListNodes(ctx context.Context) ([]domain.Node, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.id,n.agent_id,n.machine_id,m.name,n.route_policy_id,n.admin_route_policy_id,n.member_route_policy_id,n.name,n.protocol,n.listen_ip,
		       n.server_port,n.kernel_type,n.config,n.status,n.current_revision,n.applied_revision,
		       n.last_report_at,n.last_error,n.created_at,n.updated_at
		FROM nodes n JOIN machines m ON m.id=n.machine_id
		WHERE n.status <> 'archived' ORDER BY n.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Node, 0)
	for rows.Next() {
		var item domain.Node
		if err := rows.Scan(&item.ID, &item.AgentID, &item.MachineID, &item.MachineName, &item.RoutePolicyID, &item.AdminRoutePolicyID, &item.MemberRoutePolicyID,
			&item.Name, &item.Protocol, &item.ListenIP, &item.ServerPort, &item.KernelType, &item.Config,
			&item.Status, &item.CurrentRevision, &item.AppliedRevision, &item.LastReport, &item.LastError,
			&item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Endpoints, err = s.nodeEndpoints(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetNode(ctx context.Context, nodeID string) (domain.Node, error) {
	var item domain.Node
	err := s.pool.QueryRow(ctx, `
		SELECT n.id,n.agent_id,n.machine_id,m.name,n.route_policy_id,n.admin_route_policy_id,n.member_route_policy_id,n.name,n.protocol,n.listen_ip,
		       n.server_port,n.kernel_type,n.config,n.status,n.current_revision,n.applied_revision,
		       n.last_report_at,n.last_error,n.created_at,n.updated_at
		FROM nodes n JOIN machines m ON m.id=n.machine_id
		WHERE n.id=$1 AND n.status <> 'archived'`, nodeID,
	).Scan(&item.ID, &item.AgentID, &item.MachineID, &item.MachineName, &item.RoutePolicyID, &item.AdminRoutePolicyID, &item.MemberRoutePolicyID,
		&item.Name, &item.Protocol, &item.ListenIP, &item.ServerPort, &item.KernelType, &item.Config,
		&item.Status, &item.CurrentRevision, &item.AppliedRevision, &item.LastReport, &item.LastError,
		&item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Node{}, ErrNotFound
	}
	if err != nil {
		return domain.Node{}, mapError(err)
	}
	item.Endpoints, err = s.nodeEndpoints(ctx, item.ID)
	return item, err
}

func (s *Store) nodeEndpoints(ctx context.Context, nodeID string) ([]domain.NodeEndpoint, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,node_id,name,host,port,status,COALESCE(access_scope,'default'),sort_order,created_at,updated_at
		FROM node_endpoints WHERE node_id=$1 ORDER BY sort_order,name,id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.NodeEndpoint, 0)
	for rows.Next() {
		var item domain.NodeEndpoint
		if err := rows.Scan(&item.ID, &item.NodeID, &item.Name, &item.Host, &item.Port, &item.Status,
			&item.AccessScope, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func replaceNodeEndpoints(ctx context.Context, tx pgx.Tx, nodeID string, endpoints []domain.NodeEndpoint) ([]domain.NodeEndpoint, error) {
	if _, err := tx.Exec(ctx, `DELETE FROM node_endpoints WHERE node_id=$1`, nodeID); err != nil {
		return nil, err
	}
	items := make([]domain.NodeEndpoint, 0, len(endpoints))
	for index, endpoint := range endpoints {
		endpoint.ID = domain.MustID("ep")
		endpoint.NodeID = nodeID
		endpoint.Name = strings.TrimSpace(endpoint.Name)
		endpoint.Host = strings.TrimSpace(endpoint.Host)
		endpoint.SortOrder = index
		if endpoint.Status == "" {
			endpoint.Status = "active"
		}
		if endpoint.AccessScope == "" {
			endpoint.AccessScope = "default"
		}
		err := tx.QueryRow(ctx, `INSERT INTO node_endpoints(id,node_id,name,host,port,status,access_scope,sort_order)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at,updated_at`, endpoint.ID, endpoint.NodeID,
			endpoint.Name, endpoint.Host, endpoint.Port, endpoint.Status, endpoint.AccessScope, endpoint.SortOrder).Scan(&endpoint.CreatedAt, &endpoint.UpdatedAt)
		if err != nil {
			return nil, mapError(err)
		}
		items = append(items, endpoint)
	}
	return items, nil
}

func (s *Store) CreateNode(ctx context.Context, input domain.NodeCreate) (domain.Node, error) {
	item := domain.Node{ID: domain.MustID("nod")}
	input.RoutePolicyID = normalizeOptionalID(input.RoutePolicyID)
	input.AdminRoutePolicyID = normalizeOptionalID(input.AdminRoutePolicyID)
	input.MemberRoutePolicyID = normalizeOptionalID(input.MemberRoutePolicyID)
	if input.ListenIP == "" {
		input.ListenIP = "0.0.0.0"
	}
	if input.KernelType == "" {
		input.KernelType = "xray"
	}
	input.KernelType = strings.ToLower(strings.TrimSpace(input.KernelType))
	if len(input.Config) == 0 {
		input.Config = json.RawMessage(`{}`)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Node{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.Node{}, err
	}
	if err := s.validateNodeBindingsTx(ctx, tx, input.MachineID, input.RoutePolicyID, input.AdminRoutePolicyID, input.MemberRoutePolicyID, input.KernelType); err != nil {
		return domain.Node{}, err
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO nodes(id,machine_id,route_policy_id,admin_route_policy_id,member_route_policy_id,name,protocol,listen_ip,server_port,kernel_type,config)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING agent_id,status,current_revision,applied_revision,last_report_at,last_error,created_at,updated_at`,
		item.ID, input.MachineID, input.RoutePolicyID, input.AdminRoutePolicyID, input.MemberRoutePolicyID, strings.TrimSpace(input.Name), strings.ToLower(input.Protocol),
		input.ListenIP, input.ServerPort, input.KernelType, input.Config,
	).Scan(&item.AgentID, &item.Status, &item.CurrentRevision, &item.AppliedRevision, &item.LastReport,
		&item.LastError, &item.CreatedAt, &item.UpdatedAt)
	item.MachineID = input.MachineID
	item.RoutePolicyID = input.RoutePolicyID
	item.AdminRoutePolicyID = input.AdminRoutePolicyID
	item.MemberRoutePolicyID = input.MemberRoutePolicyID
	item.Name = strings.TrimSpace(input.Name)
	item.Protocol = strings.ToLower(input.Protocol)
	item.ListenIP = input.ListenIP
	item.ServerPort = input.ServerPort
	item.KernelType = input.KernelType
	item.Config = input.Config
	if err != nil {
		return domain.Node{}, mapNodeError(err)
	}
	item.Endpoints, err = replaceNodeEndpoints(ctx, tx, item.ID, input.Endpoints)
	if err != nil {
		return domain.Node{}, err
	}
	return item, tx.Commit(ctx)
}

func normalizeOptionalID(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mapNodeError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "nodes_machine_id_server_port_key", "nodes_machine_port_active_unique":
			return errors.Join(ErrNodePortInUse, err)
		}
	}
	return mapError(err)
}

func (s *Store) ArchiveNode(ctx context.Context, nodeID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return err
	}
	var machineID string
	var agentID int64
	var revision int
	err = tx.QueryRow(ctx, `UPDATE nodes SET status='archived',updated_at=now()
		WHERE id=$1 AND status <> 'archived'
		RETURNING machine_id,agent_id,current_revision`, nodeID).Scan(&machineID, &agentID, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
		VALUES($1,$2,'machine.nodes.replace',$3,$4::jsonb)`,
		machineID, nodeID, revision, machineNodesReplacePayload(agentID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func machineNodesReplacePayload(agentID int64) json.RawMessage {
	payload, _ := json.Marshal(struct {
		NodeID int64 `json:"node_id"`
	}{NodeID: agentID})
	return payload
}

func (s *Store) UpdateNode(ctx context.Context, nodeID string, input domain.NodeUpdate) (domain.Node, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Node{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.Node{}, err
	}
	var item domain.Node
	err = tx.QueryRow(ctx, `
		SELECT id,agent_id,machine_id,route_policy_id,admin_route_policy_id,member_route_policy_id,name,protocol,listen_ip,
		       server_port,kernel_type,config,status,current_revision,applied_revision,last_report_at,last_error,created_at,updated_at
		FROM nodes WHERE id=$1 AND status <> 'archived' FOR UPDATE`, nodeID).Scan(
		&item.ID, &item.AgentID, &item.MachineID, &item.RoutePolicyID, &item.AdminRoutePolicyID, &item.MemberRoutePolicyID,
		&item.Name, &item.Protocol, &item.ListenIP, &item.ServerPort, &item.KernelType, &item.Config, &item.Status,
		&item.CurrentRevision, &item.AppliedRevision, &item.LastReport, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Node{}, ErrNotFound
	}
	if err != nil {
		return domain.Node{}, err
	}
	originalMachineID := item.MachineID
	originalStatus := item.Status
	if input.MachineID != nil {
		item.MachineID = strings.TrimSpace(*input.MachineID)
	}
	if input.RoutePolicyID != nil {
		value := strings.TrimSpace(*input.RoutePolicyID)
		if value == "" {
			item.RoutePolicyID = nil
		} else {
			item.RoutePolicyID = &value
		}
	}
	if input.AdminRoutePolicyID != nil {
		value := strings.TrimSpace(*input.AdminRoutePolicyID)
		if value == "" {
			item.AdminRoutePolicyID = nil
		} else {
			item.AdminRoutePolicyID = &value
		}
	}
	if input.MemberRoutePolicyID != nil {
		value := strings.TrimSpace(*input.MemberRoutePolicyID)
		if value == "" {
			item.MemberRoutePolicyID = nil
		} else {
			item.MemberRoutePolicyID = &value
		}
	}
	if input.Name != nil {
		item.Name = strings.TrimSpace(*input.Name)
	}
	if input.Protocol != nil {
		item.Protocol = strings.ToLower(strings.TrimSpace(*input.Protocol))
	}
	if input.ListenIP != nil {
		item.ListenIP = strings.TrimSpace(*input.ListenIP)
		if item.ListenIP == "" {
			item.ListenIP = "0.0.0.0"
		}
	}
	if input.ServerPort != nil {
		item.ServerPort = *input.ServerPort
	}
	if input.KernelType != nil {
		item.KernelType = strings.ToLower(strings.TrimSpace(*input.KernelType))
	}
	if input.Config != nil {
		item.Config = *input.Config
	}
	if input.Status != nil {
		status, statusErr := normalizeNodeStatus(*input.Status)
		if statusErr != nil {
			return domain.Node{}, statusErr
		}
		item.Status = status
	} else if item.Status != "disabled" {
		item.Status = "draft"
	}
	if err := s.validateNodeBindingsTx(ctx, tx, item.MachineID, item.RoutePolicyID, item.AdminRoutePolicyID, item.MemberRoutePolicyID, item.KernelType); err != nil {
		return domain.Node{}, err
	}
	command, err := tx.Exec(ctx, `
		UPDATE nodes SET machine_id=$2,route_policy_id=$3,admin_route_policy_id=$4,member_route_policy_id=$5,name=$6,protocol=$7,listen_ip=$8,
		       server_port=$9,kernel_type=$10,config=$11,
		       status=CASE WHEN $12::text IS NOT NULL THEN $12 WHEN status='disabled' THEN status ELSE 'draft' END,updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, nodeID, item.MachineID, item.RoutePolicyID, item.AdminRoutePolicyID, item.MemberRoutePolicyID, item.Name,
		item.Protocol, item.ListenIP, item.ServerPort, item.KernelType, item.Config, input.Status)
	if err != nil {
		return domain.Node{}, mapNodeError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.Node{}, ErrNotFound
	}
	if input.Endpoints != nil {
		if _, err := replaceNodeEndpoints(ctx, tx, nodeID, *input.Endpoints); err != nil {
			return domain.Node{}, err
		}
	}
	if originalMachineID != item.MachineID || originalStatus != item.Status {
		if _, err := tx.Exec(ctx, `INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
			VALUES($1,$2,'machine.nodes.replace',$3,$4::jsonb)`,
			originalMachineID, nodeID, item.CurrentRevision, machineNodesReplacePayload(item.AgentID)); err != nil {
			return domain.Node{}, err
		}
		if originalMachineID != item.MachineID {
			if _, err := tx.Exec(ctx, `INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
				VALUES($1,$2,'machine.nodes.replace',$3,$4::jsonb)`,
				item.MachineID, nodeID, item.CurrentRevision, machineNodesReplacePayload(item.AgentID)); err != nil {
				return domain.Node{}, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Node{}, err
	}
	return s.GetNode(ctx, nodeID)
}

func (s *Store) PublishNode(ctx context.Context, nodeID, adminID string) (domain.Node, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Node{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.Node{}, err
	}
	var item domain.Node
	err = tx.QueryRow(ctx, `
		UPDATE nodes SET current_revision=current_revision+1,status='published',updated_at=now()
		WHERE id=$1 AND status <> 'archived'
		RETURNING id,agent_id,machine_id,route_policy_id,admin_route_policy_id,member_route_policy_id,name,protocol,listen_ip,server_port,kernel_type,
		          config,status,current_revision,applied_revision,last_report_at,last_error,created_at,updated_at`, nodeID,
	).Scan(&item.ID, &item.AgentID, &item.MachineID, &item.RoutePolicyID, &item.AdminRoutePolicyID, &item.MemberRoutePolicyID, &item.Name, &item.Protocol,
		&item.ListenIP, &item.ServerPort, &item.KernelType, &item.Config, &item.Status, &item.CurrentRevision,
		&item.AppliedRevision, &item.LastReport, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Node{}, ErrNotFound
	}
	if err != nil {
		return domain.Node{}, err
	}
	if err := s.validateNodeBindingsTx(ctx, tx, item.MachineID, item.RoutePolicyID, item.AdminRoutePolicyID, item.MemberRoutePolicyID, item.KernelType); err != nil {
		return domain.Node{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO node_revisions(id,node_id,revision,config,published_by) VALUES($1,$2,$3,$4,$5)`,
		domain.MustID("nrv"), item.ID, item.CurrentRevision, item.Config, adminID)
	if err != nil {
		return domain.Node{}, err
	}
	payload, _ := json.Marshal(map[string]any{"node_id": item.AgentID, "config": item.Config})
	_, err = tx.Exec(ctx, `
		INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
		VALUES($1,$2,'node.spec.replace',$3,$4)`, item.MachineID, item.ID, item.CurrentRevision, payload)
	if err != nil {
		return domain.Node{}, err
	}
	return item, tx.Commit(ctx)
}

func (s *Store) ListAccessGroups(ctx context.Context) ([]domain.AccessGroup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.id,g.name,g.status,g.notes,
		       (SELECT count(*) FROM access_group_nodes gn JOIN nodes n ON n.id=gn.node_id
		        WHERE gn.access_group_id=g.id AND n.status <> 'archived'),
		       (SELECT count(*) FROM users u LEFT JOIN plans p ON p.id=u.plan_id
		        WHERE COALESCE(u.access_group_override_id,p.access_group_id)=g.id AND u.status <> 'archived'),
		       ARRAY(SELECT gn.node_id FROM access_group_nodes gn JOIN nodes n ON n.id=gn.node_id
		             WHERE gn.access_group_id=g.id AND n.status <> 'archived' ORDER BY gn.node_id),
		       g.created_at,g.updated_at
		FROM access_groups g WHERE g.status <> 'archived' ORDER BY g.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AccessGroup, 0)
	for rows.Next() {
		var item domain.AccessGroup
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.Notes, &item.NodeCount, &item.UserCount, &item.NodeIDs, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ArchiveAccessGroup(ctx context.Context, groupID string) error {
	var exists bool
	var dependencyCount int
	if err := s.pool.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM access_groups WHERE id=$1 AND status <> 'archived'),
		(SELECT count(*) FROM plans WHERE access_group_id=$1 AND status <> 'archived') +
		(SELECT count(*) FROM users WHERE access_group_override_id=$1 AND status <> 'archived')`, groupID).Scan(&exists, &dependencyCount); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	if dependencyCount > 0 {
		return ErrAccessGroupInUse
	}
	_, err := s.pool.Exec(ctx, `UPDATE access_groups SET status='archived',updated_at=now() WHERE id=$1`, groupID)
	return err
}

func (s *Store) CreateAccessGroup(ctx context.Context, name, notes string, nodeIDs []string) (domain.AccessGroup, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.AccessGroup{}, err
	}
	defer tx.Rollback(ctx)
	item := domain.AccessGroup{ID: domain.MustID("grp")}
	err = tx.QueryRow(ctx, `INSERT INTO access_groups(id,name,notes) VALUES($1,$2,$3)
		RETURNING name,status,notes,created_at,updated_at`, item.ID, strings.TrimSpace(name), strings.TrimSpace(notes),
	).Scan(&item.Name, &item.Status, &item.Notes, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return domain.AccessGroup{}, mapError(err)
	}
	for _, nodeID := range nodeIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO access_group_nodes(access_group_id,node_id) VALUES($1,$2)`, item.ID, nodeID); err != nil {
			return domain.AccessGroup{}, mapError(err)
		}
	}
	item.NodeCount = len(nodeIDs)
	item.NodeIDs = append([]string(nil), nodeIDs...)
	return item, tx.Commit(ctx)
}

func (s *Store) ArchivePlan(ctx context.Context, planID string) error {
	var exists bool
	var userCount int
	if err := s.pool.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM plans WHERE id=$1 AND status <> 'archived'),
		(SELECT count(*) FROM users WHERE plan_id=$1 AND status <> 'archived')`, planID).Scan(&exists, &userCount); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	if userCount > 0 {
		return ErrPlanInUse
	}
	_, err := s.pool.Exec(ctx, `UPDATE plans SET status='archived',updated_at=now() WHERE id=$1`, planID)
	return err
}

func (s *Store) UpdateAccessGroup(ctx context.Context, groupID string, input domain.AccessGroupUpdate) (domain.AccessGroup, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.AccessGroup{}, err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE access_groups SET
		name=COALESCE(NULLIF(BTRIM($2),''),name),notes=COALESCE($3,notes),status=COALESCE($4,status),updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, groupID, input.Name, input.Notes, input.Status)
	if err != nil {
		return domain.AccessGroup{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.AccessGroup{}, ErrNotFound
	}
	if input.NodeIDs != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM access_group_nodes WHERE access_group_id=$1`, groupID); err != nil {
			return domain.AccessGroup{}, err
		}
		for _, nodeID := range *input.NodeIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO access_group_nodes(access_group_id,node_id) VALUES($1,$2)`, groupID, nodeID); err != nil {
				return domain.AccessGroup{}, mapError(err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AccessGroup{}, err
	}
	if err := s.NotifyAllPublishedNodes(ctx); err != nil {
		return domain.AccessGroup{}, err
	}
	items, err := s.ListAccessGroups(ctx)
	if err != nil {
		return domain.AccessGroup{}, err
	}
	for _, item := range items {
		if item.ID == groupID {
			return item, nil
		}
	}
	return domain.AccessGroup{}, ErrNotFound
}

func (s *Store) ListPlans(ctx context.Context) ([]domain.Plan, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id,p.access_group_id,g.name,p.name,p.status,p.traffic_limit_bytes,p.speed_limit_mbps,
		       p.device_limit,p.reset_strategy,p.default_valid_days,p.notes,
		       (SELECT count(*) FROM users u WHERE u.plan_id=p.id AND u.status <> 'archived'),
		       p.created_at,p.updated_at
		FROM plans p JOIN access_groups g ON g.id=p.access_group_id
		WHERE p.status <> 'archived' ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Plan, 0)
	for rows.Next() {
		var item domain.Plan
		if err := rows.Scan(&item.ID, &item.AccessGroupID, &item.AccessGroupName, &item.Name, &item.Status,
			&item.TrafficLimitBytes, &item.SpeedLimitMbps, &item.DeviceLimit, &item.ResetStrategy,
			&item.DefaultValidDays, &item.Notes, &item.UserCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreatePlan(ctx context.Context, input domain.PlanCreate) (domain.Plan, error) {
	if input.ResetStrategy == "" {
		input.ResetStrategy = "calendar_month"
	}
	item := domain.Plan{ID: domain.MustID("pln"), ResetStrategy: input.ResetStrategy}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO plans(id,access_group_id,name,traffic_limit_bytes,speed_limit_mbps,device_limit,reset_strategy,default_valid_days,notes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING name,status,traffic_limit_bytes,speed_limit_mbps,device_limit,reset_strategy,
		          default_valid_days,notes,created_at,updated_at`,
		item.ID, input.AccessGroupID, strings.TrimSpace(input.Name), input.TrafficLimitBytes,
		input.SpeedLimitMbps, input.DeviceLimit, input.ResetStrategy, input.DefaultValidDays, strings.TrimSpace(input.Notes),
	).Scan(&item.Name, &item.Status, &item.TrafficLimitBytes, &item.SpeedLimitMbps, &item.DeviceLimit,
		&item.ResetStrategy, &item.DefaultValidDays, &item.Notes, &item.CreatedAt, &item.UpdatedAt)
	item.AccessGroupID = input.AccessGroupID
	return item, mapError(err)
}

func (s *Store) UpdatePlan(ctx context.Context, planID string, input domain.PlanUpdate) (domain.Plan, error) {
	command, err := s.pool.Exec(ctx, `UPDATE plans SET
		access_group_id=COALESCE(NULLIF(BTRIM($2),''),access_group_id),name=COALESCE(NULLIF(BTRIM($3),''),name),
		traffic_limit_bytes=COALESCE($4,traffic_limit_bytes),speed_limit_mbps=COALESCE($5,speed_limit_mbps),
		device_limit=COALESCE($6,device_limit),reset_strategy=COALESCE($7,reset_strategy),
		default_valid_days=COALESCE($8,default_valid_days),notes=COALESCE($9,notes),
		status=COALESCE($10,status),updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, planID, input.AccessGroupID, input.Name, input.TrafficLimitBytes,
		input.SpeedLimitMbps, input.DeviceLimit, input.ResetStrategy, input.DefaultValidDays, input.Notes, input.Status)
	if err != nil {
		return domain.Plan{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.Plan{}, ErrNotFound
	}
	if _, err := s.ReconcileTrafficUsage(ctx); err != nil {
		return domain.Plan{}, err
	}
	if err := s.NotifyAllPublishedNodes(ctx); err != nil {
		return domain.Plan{}, err
	}
	items, err := s.ListPlans(ctx)
	if err != nil {
		return domain.Plan{}, err
	}
	for _, item := range items {
		if item.ID == planID {
			return item, nil
		}
	}
	return domain.Plan{}, ErrNotFound
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.User, error) {
	if err := s.EnsureAdminUsers(ctx); err != nil {
		return nil, err
	}
	if _, err := s.ReconcileTrafficUsage(ctx); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT u.id,u.agent_id,u.role,u.plan_id,p.name,u.access_group_override_id,u.name,u.email,u.uuid,
		       u.subscription_token_prefix,(COALESCE(u.subscription_token_plain,'') <> ''),u.status,
		       COALESCE(u.portal_login,''),(u.portal_password_hash IS NOT NULL),
		       u.traffic_limit_override_bytes,u.speed_limit_override_mbps,
		       u.device_limit_override,u.traffic_used_bytes,
		       COALESCE(u.traffic_limit_override_bytes,p.traffic_limit_bytes,0),
		       u.traffic_reset_at,u.expires_at,u.notes,u.created_at,u.updated_at
		FROM users u LEFT JOIN plans p ON p.id=u.plan_id
		WHERE u.status <> 'archived' ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.User, 0)
	for rows.Next() {
		var item domain.User
		if err := rows.Scan(&item.ID, &item.AgentID, &item.Role, &item.PlanID, &item.PlanName,
			&item.AccessGroupOverrideID, &item.Name, &item.Email, &item.UUID, &item.SubscriptionTokenPrefix,
			&item.SubscriptionAvailable, &item.Status, &item.PortalLogin, &item.PortalEnabled,
			&item.TrafficLimitOverrideBytes, &item.SpeedLimitOverrideMbps,
			&item.DeviceLimitOverride, &item.TrafficUsedBytes, &item.TrafficLimitBytes, &item.TrafficResetAt, &item.ExpiresAt,
			&item.Notes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, input domain.UserCreate) (domain.UserCreated, error) {
	uuid, err := domain.NewUUID()
	if err != nil {
		return domain.UserCreated{}, err
	}
	plain, hash, err := auth.NewSecret("cps_", 32)
	if err != nil {
		return domain.UserCreated{}, err
	}
	item := domain.UserCreated{User: domain.User{ID: domain.MustID("usr"), UUID: uuid}, SubscriptionToken: plain}
	if input.Role == "" {
		input.Role = "user"
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users(id,role,plan_id,access_group_override_id,name,email,uuid,subscription_token_hash,
		                  subscription_token_prefix,subscription_token_plain,traffic_limit_override_bytes,speed_limit_override_mbps,
		                  device_limit_override,expires_at,notes,portal_login,portal_password_hash,portal_enabled_at,traffic_reset_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,
		       NULLIF(BTRIM($16),''),NULLIF($17,''),CASE WHEN NULLIF($17,'') IS NULL THEN NULL ELSE now() END,
		       date_trunc('month',now()) + interval '1 month')
		RETURNING agent_id,role,plan_id,access_group_override_id,name,email,subscription_token_prefix,status,
		          COALESCE(portal_login,''),(portal_password_hash IS NOT NULL),
		          traffic_limit_override_bytes,speed_limit_override_mbps,device_limit_override,traffic_used_bytes,
		          traffic_reset_at,expires_at,notes,created_at,updated_at`,
		item.ID, input.Role, input.PlanID, input.AccessGroupOverrideID, strings.TrimSpace(input.Name), input.Email,
		uuid, hash, auth.Prefix(plain, 12), plain, input.TrafficLimitOverrideBytes, input.SpeedLimitOverrideMbps,
		input.DeviceLimitOverride, input.ExpiresAt, strings.TrimSpace(input.Notes), input.PortalLogin, input.PortalPasswordHash,
	).Scan(&item.AgentID, &item.Role, &item.PlanID, &item.AccessGroupOverrideID, &item.Name, &item.Email,
		&item.SubscriptionTokenPrefix, &item.Status, &item.PortalLogin, &item.PortalEnabled,
		&item.TrafficLimitOverrideBytes, &item.SpeedLimitOverrideMbps,
		&item.DeviceLimitOverride, &item.TrafficUsedBytes, &item.TrafficResetAt, &item.ExpiresAt, &item.Notes,
		&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return item, mapError(err)
	}
	if err := s.NotifyAllPublishedNodes(ctx); err != nil {
		return domain.UserCreated{}, err
	}
	return item, nil
}

func (s *Store) UpdateUser(ctx context.Context, userID string, input domain.UserUpdate) (domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)
	var adminLinked bool
	if err := tx.QueryRow(ctx, `SELECT admin_id IS NOT NULL FROM users
		WHERE id=$1 AND status <> 'archived'`, userID).Scan(&adminLinked); errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	} else if err != nil {
		return domain.User{}, err
	}
	if adminLinked && ((input.Role != nil && *input.Role != "admin") || input.Status != nil || input.PortalLogin != nil || input.PortalPasswordHash != nil) {
		return domain.User{}, ErrConflict
	}
	if !adminLinked && input.Role != nil && *input.Role == "admin" {
		return domain.User{}, ErrConflict
	}
	command, err := tx.Exec(ctx, `UPDATE users SET
		role=COALESCE($2,role),uuid=COALESCE(NULLIF(BTRIM($3),''),uuid),
		plan_id=CASE WHEN $4::text IS NULL THEN plan_id ELSE NULLIF(BTRIM($4),'') END,
		access_group_override_id=CASE WHEN $5::text IS NULL THEN access_group_override_id ELSE NULLIF(BTRIM($5),'') END,
		name=COALESCE(NULLIF(BTRIM($6),''),name),email=CASE WHEN $7::text IS NULL THEN email ELSE NULLIF(BTRIM($7),'') END,
		expires_at=CASE WHEN $8::text IS NULL THEN expires_at ELSE NULLIF(BTRIM($8),'')::timestamptz END,
		notes=COALESCE($9,notes),status=COALESCE($10,status),
		portal_login=CASE WHEN $11::text IS NULL THEN portal_login ELSE NULLIF(BTRIM($11),'') END,
		portal_password_hash=COALESCE(NULLIF($12,''),portal_password_hash),
		portal_enabled_at=CASE WHEN NULLIF($12,'') IS NULL THEN portal_enabled_at ELSE now() END,
		updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, userID, input.Role, input.UUID, input.PlanID, input.AccessGroupOverrideID,
		input.Name, input.Email, input.ExpiresAt, input.Notes, input.Status, input.PortalLogin, input.PortalPasswordHash)
	if err != nil {
		return domain.User{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.User{}, ErrNotFound
	}
	if input.PortalPasswordHash != nil && *input.PortalPasswordHash != "" {
		if _, err := tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=now()
			WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
			return domain.User{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	if err := s.NotifyAllPublishedNodes(ctx); err != nil {
		return domain.User{}, err
	}
	items, err := s.ListUsers(ctx)
	if err != nil {
		return domain.User{}, err
	}
	for _, item := range items {
		if item.ID == userID {
			return item, nil
		}
	}
	return domain.User{}, ErrNotFound
}

func (s *Store) UserSubscriptionToken(ctx context.Context, userID string) (string, error) {
	var token string
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(subscription_token_plain,'') FROM users
		WHERE id=$1 AND status <> 'archived'`, userID).Scan(&token)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", ErrSubscriptionTokenUnavailable
	}
	return token, nil
}

func (s *Store) ArchiveUser(ctx context.Context, userID string) error {
	var adminLinked bool
	if err := s.pool.QueryRow(ctx, `SELECT admin_id IS NOT NULL FROM users
		WHERE id=$1 AND status <> 'archived'`, userID).Scan(&adminLinked); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if adminLinked {
		return ErrConflict
	}
	command, err := s.pool.Exec(ctx, `UPDATE users SET status='archived',updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return s.NotifyAllPublishedNodes(ctx)
}

func (s *Store) ListRoutePolicies(ctx context.Context) ([]domain.RoutePolicy, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id,r.name,r.status,r.scope,r.current_revision,r.default_outbound_tag,
		       COALESCE((SELECT rr.rules FROM route_policy_revisions rr
		                 WHERE rr.route_policy_id=r.id ORDER BY rr.revision DESC LIMIT 1),'[]'::jsonb),
		       r.notes,
		       (SELECT count(*) FROM nodes n WHERE r.id IN (n.route_policy_id,n.admin_route_policy_id,n.member_route_policy_id) AND n.status <> 'archived'),
		       r.created_at,r.updated_at
		FROM route_policies r WHERE r.status <> 'archived' ORDER BY r.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RoutePolicy, 0)
	for rows.Next() {
		var item domain.RoutePolicy
		var rawRules []byte
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.Scope, &item.CurrentRevision, &item.DefaultOutboundTag, &rawRules, &item.Notes,
			&item.NodeCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rawRules, &item.Rules); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateRoutePolicy(ctx context.Context, name, notes, scope, defaultOutboundTag, adminID string, rules []domain.RoutePolicyRule) (domain.RoutePolicy, error) {
	scope = normalizeRoutePolicyScope(scope)
	defaultOutboundTag = strings.ToLower(strings.TrimSpace(defaultOutboundTag))
	rawRules, err := json.Marshal(rules)
	if err != nil {
		return domain.RoutePolicy{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.RoutePolicy{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.RoutePolicy{}, err
	}
	if err := validateRouteTargetsAgainstOutbounds(ctx, tx, rules, defaultOutboundTag, nil); err != nil {
		return domain.RoutePolicy{}, err
	}
	item := domain.RoutePolicy{ID: domain.MustID("rte"), Rules: rules, Scope: scope}
	err = tx.QueryRow(ctx, `INSERT INTO route_policies(id,name,notes,scope,default_outbound_tag,status,current_revision) VALUES($1,$2,$3,$4,$5,'published',1)
		RETURNING name,status,scope,current_revision,default_outbound_tag,notes,created_at,updated_at`,
		item.ID, strings.TrimSpace(name), strings.TrimSpace(notes), scope, defaultOutboundTag,
	).Scan(&item.Name, &item.Status, &item.Scope, &item.CurrentRevision, &item.DefaultOutboundTag, &item.Notes, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return domain.RoutePolicy{}, mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO route_policy_revisions(id,route_policy_id,revision,rules,published_by)
		VALUES($1,$2,1,$3,$4)`, domain.MustID("rrv"), item.ID, rawRules, adminID); err != nil {
		return domain.RoutePolicy{}, mapError(err)
	}
	return item, tx.Commit(ctx)
}

func (s *Store) ArchiveRoutePolicy(ctx context.Context, routeID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM route_policies WHERE id=$1 FOR UPDATE`, routeID).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if status == "archived" {
		return ErrNotFound
	}
	var nodeCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM nodes WHERE $1 IN (route_policy_id,admin_route_policy_id,member_route_policy_id) AND status <> 'archived'`, routeID).Scan(&nodeCount); err != nil {
		return err
	}
	if nodeCount > 0 {
		return ErrRoutePolicyInUse
	}
	if _, err := tx.Exec(ctx, `UPDATE route_policies SET status='archived',updated_at=now() WHERE id=$1`, routeID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateRoutePolicy(ctx context.Context, routeID, adminID string, input domain.RoutePolicyUpdate) (domain.RoutePolicy, error) {
	if input.DefaultOutboundTag != nil {
		value := strings.ToLower(strings.TrimSpace(*input.DefaultOutboundTag))
		input.DefaultOutboundTag = &value
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.RoutePolicy{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.RoutePolicy{}, err
	}
	var currentRevision int
	var currentStatus, currentScope, currentDefaultTag string
	var rawRules []byte
	err = tx.QueryRow(ctx, `SELECT current_revision,status,scope,COALESCE(default_outbound_tag,''),
		COALESCE((SELECT rr.rules FROM route_policy_revisions rr WHERE rr.route_policy_id=r.id ORDER BY rr.revision DESC LIMIT 1),'[]'::jsonb)
		FROM route_policies r WHERE id=$1 AND status <> 'archived' FOR UPDATE`, routeID).Scan(&currentRevision, &currentStatus, &currentScope, &currentDefaultTag, &rawRules)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RoutePolicy{}, ErrNotFound
	}
	if err != nil {
		return domain.RoutePolicy{}, err
	}
	var currentRules []domain.RoutePolicyRule
	if err := json.Unmarshal(rawRules, &currentRules); err != nil {
		return domain.RoutePolicy{}, err
	}
	resultingStatus, resultingScope, resultingDefaultTag := currentStatus, normalizeRoutePolicyScope(currentScope), currentDefaultTag
	if input.Status != nil {
		resultingStatus, err = normalizeRoutePolicyStatus(*input.Status)
		if err != nil {
			return domain.RoutePolicy{}, err
		}
	}
	if input.Scope != nil {
		resultingScope = normalizeRoutePolicyScope(*input.Scope)
	}
	if input.DefaultOutboundTag != nil {
		resultingDefaultTag = *input.DefaultOutboundTag
	}
	resultingRules := currentRules
	if input.Rules != nil {
		resultingRules = *input.Rules
	}
	if err := s.validateRoutePolicyMutationTx(ctx, tx, routeID, resultingStatus, resultingScope, resultingDefaultTag, resultingRules); err != nil {
		return domain.RoutePolicy{}, err
	}
	var updatedRevision = currentRevision
	if input.Rules != nil || input.DefaultOutboundTag != nil || input.Scope != nil || input.Status != nil {
		updatedRevision++
	}
	_, err = tx.Exec(ctx, `UPDATE route_policies SET
		name=COALESCE(NULLIF(BTRIM($2),''),name),notes=COALESCE($3,notes),status=$4,scope=$5,
		default_outbound_tag=$6,current_revision=$7,updated_at=now()
		WHERE id=$1`, routeID, input.Name, input.Notes, resultingStatus, resultingScope, resultingDefaultTag, updatedRevision)
	if err != nil {
		return domain.RoutePolicy{}, mapError(err)
	}
	if input.Rules != nil || input.DefaultOutboundTag != nil || input.Scope != nil || input.Status != nil {
		rawRules, err := json.Marshal(resultingRules)
		if err != nil {
			return domain.RoutePolicy{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO route_policy_revisions(id,route_policy_id,revision,rules,published_by)
			VALUES($1,$2,$3,$4,$5)`, domain.MustID("rrv"), routeID, updatedRevision, rawRules, adminID); err != nil {
			return domain.RoutePolicy{}, mapError(err)
		}
	}
	if input.Rules != nil || input.Status != nil || input.DefaultOutboundTag != nil || input.Scope != nil {
		if err := notifyRoutePolicyNodes(ctx, tx, routeID); err != nil {
			return domain.RoutePolicy{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RoutePolicy{}, err
	}
	items, err := s.ListRoutePolicies(ctx)
	if err != nil {
		return domain.RoutePolicy{}, err
	}
	for _, item := range items {
		if item.ID == routeID {
			return item, nil
		}
	}
	return domain.RoutePolicy{}, ErrNotFound
}

func (s *Store) routePolicyTargets(ctx context.Context, routeID string) ([]domain.RoutePolicyRule, string, error) {
	var rawRules []byte
	var defaultOutboundTag string
	err := s.pool.QueryRow(ctx, `SELECT r.default_outbound_tag,
		COALESCE((SELECT rr.rules FROM route_policy_revisions rr WHERE rr.route_policy_id=r.id ORDER BY rr.revision DESC LIMIT 1),'[]'::jsonb)
		FROM route_policies r WHERE r.id=$1 AND r.status <> 'archived'`, routeID).Scan(&defaultOutboundTag, &rawRules)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	var rules []domain.RoutePolicyRule
	if err := json.Unmarshal(rawRules, &rules); err != nil {
		return nil, "", err
	}
	return rules, strings.ToLower(strings.TrimSpace(defaultOutboundTag)), nil
}

func normalizeRoutePolicyScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "admin":
		return "admin"
	case "member", "user", "friend":
		return "member"
	default:
		return "default"
	}
}

func (s *Store) validateRoutePolicyTargets(ctx context.Context, rules []domain.RoutePolicyRule, defaultOutboundTag string) error {
	return validateRouteTargetsAgainstOutbounds(ctx, s.pool, rules, defaultOutboundTag, nil)
}

func notifyRoutePolicyNodes(ctx context.Context, tx pgx.Tx, routeID string) error {
	_, err := tx.Exec(ctx, `WITH changed AS (
		UPDATE nodes SET current_revision=current_revision+1,updated_at=now()
		WHERE $1 IN (route_policy_id,admin_route_policy_id,member_route_policy_id) AND status='published'
		RETURNING id,agent_id,machine_id,current_revision
	)
	INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
	SELECT machine_id,id,'node.spec.replace',current_revision,jsonb_build_object('node_id',agent_id)
	FROM changed`, routeID)
	return err
}

func (s *Store) ListOutbounds(ctx context.Context) ([]domain.Outbound, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,tag,protocol,settings,proxy_tag,kernel_support,status,created_at,updated_at
		FROM outbounds WHERE status <> 'archived' ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Outbound, 0)
	for rows.Next() {
		var item domain.Outbound
		if err := rows.Scan(&item.ID, &item.Name, &item.Tag, &item.Protocol, &item.Settings, &item.ProxyTag,
			&item.KernelSupport, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ArchiveOutbound(ctx context.Context, outboundID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return err
	}

	var current outboundReference
	err = tx.QueryRow(ctx, `SELECT lower(tag),protocol,status,lower(NULLIF(proxy_tag,'')),kernel_support
		FROM outbounds WHERE id=$1 AND status <> 'archived' FOR UPDATE`, outboundID).Scan(
		&current.Tag, &current.Protocol, &current.Status, &current.ProxyTag, &current.KernelSupport)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	inUse, err := s.outboundTagReferencedTx(ctx, tx, current.Tag)
	if err != nil {
		return err
	}
	if inUse {
		return ErrOutboundInUse
	}
	if _, err := tx.Exec(ctx, `UPDATE outbounds SET status='archived',updated_at=now() WHERE id=$1`, outboundID); err != nil {
		return err
	}
	if err := notifyAllPublishedNodeSpecs(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CreateOutbound(ctx context.Context, input domain.Outbound) (domain.Outbound, error) {
	input.ID = domain.MustID("out")
	if len(input.Settings) == 0 {
		input.Settings = json.RawMessage(`{}`)
	}
	if len(input.KernelSupport) == 0 {
		input.KernelSupport = []string{"xray"}
	}
	input.Tag = strings.ToLower(strings.TrimSpace(input.Tag))
	input.Protocol = strings.ToLower(strings.TrimSpace(input.Protocol))
	input.ProxyTag = strings.ToLower(strings.TrimSpace(input.ProxyTag))
	if input.Status != "" && strings.ToLower(strings.TrimSpace(input.Status)) != "active" {
		return domain.Outbound{}, ErrConflict
	}
	if input.Tag == "" || input.Tag == "direct" || input.Tag == "block" {
		return domain.Outbound{}, ErrConflict
	}
	if input.Protocol == "" {
		return domain.Outbound{}, ErrConflict
	}
	if err := domain.ValidateOutbound(input.Tag, input.Protocol, input.Settings); err != nil {
		return domain.Outbound{}, errors.Join(ErrConflict, err)
	}
	kernels, err := normalizeKernelSupport(input.KernelSupport)
	if err != nil {
		return domain.Outbound{}, err
	}
	input.KernelSupport = kernels
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Outbound{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.Outbound{}, err
	}
	outbounds, err := loadOutboundReferences(ctx, tx)
	if err != nil {
		return domain.Outbound{}, err
	}
	candidate := outboundReference{Tag: input.Tag, Protocol: input.Protocol, Status: "active", ProxyTag: input.ProxyTag, KernelSupport: input.KernelSupport}
	outbounds[candidate.Tag] = candidate
	if err := validateOutboundChain(candidate.Tag, outbounds, nil, map[string]bool{}); err != nil {
		return domain.Outbound{}, err
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO outbounds(id,name,tag,protocol,settings,proxy_tag,kernel_support)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING status,created_at,updated_at`,
		input.ID, strings.TrimSpace(input.Name), strings.TrimSpace(input.Tag), strings.ToLower(input.Protocol),
		input.Settings, strings.TrimSpace(input.ProxyTag), input.KernelSupport,
	).Scan(&input.Status, &input.CreatedAt, &input.UpdatedAt)
	if err != nil {
		return domain.Outbound{}, mapError(err)
	}
	return input, tx.Commit(ctx)
}

func (s *Store) UpdateOutbound(ctx context.Context, outboundID string, input domain.OutboundUpdate) (domain.Outbound, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Outbound{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockResourceConsistencyTx(ctx, tx); err != nil {
		return domain.Outbound{}, err
	}
	var current outboundReference
	var currentID string
	err = tx.QueryRow(ctx, `SELECT id,lower(tag),protocol,status,lower(NULLIF(proxy_tag,'')),kernel_support
		FROM outbounds WHERE id=$1 AND status <> 'archived' FOR UPDATE`, outboundID).Scan(
		&currentID, &current.Tag, &current.Protocol, &current.Status, &current.ProxyTag, &current.KernelSupport)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Outbound{}, ErrNotFound
	}
	if err != nil {
		return domain.Outbound{}, err
	}
	resulting := current
	if input.Tag != nil {
		resulting.Tag = strings.ToLower(strings.TrimSpace(*input.Tag))
	}
	if input.Protocol != nil {
		resulting.Protocol = strings.ToLower(strings.TrimSpace(*input.Protocol))
	}
	if input.ProxyTag != nil {
		resulting.ProxyTag = strings.ToLower(strings.TrimSpace(*input.ProxyTag))
	}
	if input.KernelSupport != nil {
		kernels, normalizeErr := normalizeKernelSupport(*input.KernelSupport)
		if normalizeErr != nil {
			return domain.Outbound{}, normalizeErr
		}
		resulting.KernelSupport = kernels
	}
	if input.Status != nil {
		resulting.Status = strings.ToLower(strings.TrimSpace(*input.Status))
	}
	if resulting.Tag == "" || resulting.Tag == "direct" || resulting.Tag == "block" {
		return domain.Outbound{}, ErrConflict
	}
	if resulting.Status != "active" && resulting.Status != "disabled" {
		return domain.Outbound{}, ErrConflict
	}
	var resultingSettings json.RawMessage
	if input.Settings != nil {
		resultingSettings = *input.Settings
	} else {
		if err := tx.QueryRow(ctx, `SELECT settings FROM outbounds WHERE id=$1`, outboundID).Scan(&resultingSettings); err != nil {
			return domain.Outbound{}, err
		}
	}
	if err := domain.ValidateOutbound(resulting.Tag, resulting.Protocol, resultingSettings); err != nil {
		return domain.Outbound{}, errors.Join(ErrConflict, err)
	}
	if input.Tag != nil {
		input.Tag = &resulting.Tag
	}
	if input.Protocol != nil {
		input.Protocol = &resulting.Protocol
	}
	if input.ProxyTag != nil {
		input.ProxyTag = &resulting.ProxyTag
	}
	if input.KernelSupport != nil {
		input.KernelSupport = &resulting.KernelSupport
	}
	if input.Status != nil {
		input.Status = &resulting.Status
	}
	if resulting.Tag != current.Tag {
		// A tag is an application-level reference, so hold the outbound row and
		// validate all policy/chain references before changing it.
		if err := s.validateOutboundMutationTx(ctx, tx, resulting, current.Tag); err != nil {
			return domain.Outbound{}, err
		}
	} else if resulting.Status != current.Status || input.KernelSupport != nil || input.ProxyTag != nil {
		if err := s.validateOutboundMutationTx(ctx, tx, resulting, current.Tag); err != nil {
			return domain.Outbound{}, err
		}
	}
	command, err := tx.Exec(ctx, `UPDATE outbounds SET
		name=COALESCE(NULLIF(BTRIM($2),''),name),tag=COALESCE(NULLIF(BTRIM($3),''),tag),
		protocol=COALESCE($4,protocol),settings=COALESCE($5,settings),proxy_tag=COALESCE($6,proxy_tag),
		kernel_support=COALESCE($7,kernel_support),status=COALESCE($8,status),updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, outboundID, input.Name, input.Tag, input.Protocol,
		input.Settings, resulting.ProxyTag, resulting.KernelSupport, input.Status)
	if err != nil {
		return domain.Outbound{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.Outbound{}, ErrNotFound
	}
	if outboundUpdateAffectsNodeSpecs(input) {
		if err := notifyAllPublishedNodeSpecs(ctx, tx); err != nil {
			return domain.Outbound{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Outbound{}, err
	}
	items, err := s.ListOutbounds(ctx)
	if err != nil {
		return domain.Outbound{}, err
	}
	for _, item := range items {
		if item.ID == outboundID {
			return item, nil
		}
	}
	return domain.Outbound{}, ErrNotFound
}

func outboundUpdateAffectsNodeSpecs(input domain.OutboundUpdate) bool {
	return input.Tag != nil || input.Protocol != nil || input.Settings != nil || input.ProxyTag != nil ||
		input.KernelSupport != nil || input.Status != nil
}

func notifyAllPublishedNodeSpecs(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `WITH changed AS (
		UPDATE nodes SET current_revision=current_revision+1,updated_at=now()
		WHERE status='published'
		RETURNING id,agent_id,machine_id,current_revision
	)
	INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
	SELECT machine_id,id,'node.spec.replace',current_revision,jsonb_build_object('node_id',agent_id)
	FROM changed`)
	return err
}

func (s *Store) NotifyAllPublishedNodes(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
		SELECT machine_id,id,'node.members.replace',current_revision,jsonb_build_object('node_id',agent_id)
		FROM nodes WHERE status='published'`)
	return err
}

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
)

func (s *Store) Overview(ctx context.Context) (domain.Overview, error) {
	var overview domain.Overview
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
			COALESCE((SELECT sum(upload_bytes) FROM traffic_daily WHERE day=current_date), 0) +
			  COALESCE((SELECT sum(upload_bytes) FROM imported_user_traffic_daily WHERE day=current_date), 0),
			COALESCE((SELECT sum(download_bytes) FROM traffic_daily WHERE day=current_date), 0) +
			  COALESCE((SELECT sum(download_bytes) FROM imported_user_traffic_daily WHERE day=current_date), 0)
	`).Scan(
		&overview.MachinesTotal, &overview.MachinesOnline, &overview.MachinesOffline,
		&overview.NodesTotal, &overview.NodesPublished, &overview.AdminsActive, &overview.UsersActive,
		&overview.FriendsActive, &overview.TrafficTodayUpload, &overview.TrafficTodayDownload,
	)
	if err != nil {
		return overview, err
	}
	overview.TrafficToday = overview.TrafficTodayUpload + overview.TrafficTodayDownload
	overview.NodeTrafficRanking, err = s.trafficRanking(ctx, `WITH combined AS (
		SELECT node_id,upload_bytes,download_bytes FROM traffic_daily WHERE day=current_date
		UNION ALL SELECT node_id,upload_bytes,download_bytes FROM imported_node_traffic_daily WHERE day=current_date)
		SELECT n.id,n.name,sum(t.upload_bytes),sum(t.download_bytes),sum(t.upload_bytes+t.download_bytes)
		FROM combined t JOIN nodes n ON n.id=t.node_id GROUP BY n.id,n.name
		ORDER BY sum(t.upload_bytes+t.download_bytes) DESC,n.name LIMIT 10`)
	if err != nil {
		return overview, err
	}
	overview.UserTrafficRanking, err = s.trafficRanking(ctx, `WITH combined AS (
		SELECT user_id,upload_bytes,download_bytes FROM traffic_daily WHERE day=current_date
		UNION ALL SELECT user_id,upload_bytes,download_bytes FROM imported_user_traffic_daily WHERE day=current_date)
		SELECT u.id,u.name,sum(t.upload_bytes),sum(t.download_bytes),sum(t.upload_bytes+t.download_bytes)
		FROM combined t JOIN users u ON u.id=t.user_id GROUP BY u.id,u.name
		ORDER BY sum(t.upload_bytes+t.download_bytes) DESC,u.name LIMIT 10`)
	return overview, err
}

func (s *Store) trafficRanking(ctx context.Context, query string) ([]domain.TrafficRank, error) {
	rows, err := s.pool.Query(ctx, query)
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
	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.name, m.region, m.host, m.labels, m.notes, m.status,
		       m.agent_version, m.kernel_type, m.capabilities, m.last_heartbeat_at,
		       (SELECT count(*) FROM nodes n WHERE n.machine_id=m.id AND n.status <> 'archived'),
		       m.created_at, m.updated_at
		FROM machines m WHERE m.status <> 'archived'
		ORDER BY m.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Machine, 0)
	for rows.Next() {
		var item domain.Machine
		if err := rows.Scan(&item.ID, &item.Name, &item.Region, &item.Host, &item.Labels, &item.Notes,
			&item.Status, &item.AgentVersion, &item.KernelType, &item.Capabilities, &item.LastHeartbeat,
			&item.NodeCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateMachine(ctx context.Context, input domain.MachineCreate) (domain.Machine, error) {
	item := domain.Machine{ID: domain.MustID("mch")}
	labels := input.Labels
	if len(labels) == 0 {
		labels = json.RawMessage(`{}`)
	}
	if input.KernelType == "" {
		input.KernelType = "singbox"
	}
	err := s.pool.QueryRow(ctx, `
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

func (s *Store) UpdateMachine(ctx context.Context, machineID string, input domain.MachineUpdate) (domain.Machine, error) {
	command, err := s.pool.Exec(ctx, `UPDATE machines SET
		name=COALESCE(NULLIF(BTRIM($2),''),name),region=COALESCE($3,region),host=COALESCE($4,host),
		labels=COALESCE($5,labels),notes=COALESCE($6,notes),kernel_type=COALESCE($7,kernel_type),
		status=COALESCE($8,status),updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, machineID, input.Name, input.Region, input.Host,
		input.Labels, input.Notes, input.KernelType, input.Status)
	if err != nil {
		return domain.Machine{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.Machine{}, ErrNotFound
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
		SELECT n.id,n.agent_id,n.machine_id,m.name,n.route_policy_id,n.name,n.protocol,n.listen_ip,
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
		if err := rows.Scan(&item.ID, &item.AgentID, &item.MachineID, &item.MachineName, &item.RoutePolicyID,
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
		SELECT n.id,n.agent_id,n.machine_id,m.name,n.route_policy_id,n.name,n.protocol,n.listen_ip,
		       n.server_port,n.kernel_type,n.config,n.status,n.current_revision,n.applied_revision,
		       n.last_report_at,n.last_error,n.created_at,n.updated_at
		FROM nodes n JOIN machines m ON m.id=n.machine_id
		WHERE n.id=$1 AND n.status <> 'archived'`, nodeID,
	).Scan(&item.ID, &item.AgentID, &item.MachineID, &item.MachineName, &item.RoutePolicyID,
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
	rows, err := s.pool.Query(ctx, `SELECT id,node_id,name,host,port,status,sort_order,created_at,updated_at
		FROM node_endpoints WHERE node_id=$1 ORDER BY sort_order,name,id`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.NodeEndpoint, 0)
	for rows.Next() {
		var item domain.NodeEndpoint
		if err := rows.Scan(&item.ID, &item.NodeID, &item.Name, &item.Host, &item.Port, &item.Status,
			&item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
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
		err := tx.QueryRow(ctx, `INSERT INTO node_endpoints(id,node_id,name,host,port,status,sort_order)
			VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING created_at,updated_at`, endpoint.ID, endpoint.NodeID,
			endpoint.Name, endpoint.Host, endpoint.Port, endpoint.Status, endpoint.SortOrder).Scan(&endpoint.CreatedAt, &endpoint.UpdatedAt)
		if err != nil {
			return nil, mapError(err)
		}
		items = append(items, endpoint)
	}
	return items, nil
}

func (s *Store) CreateNode(ctx context.Context, input domain.NodeCreate) (domain.Node, error) {
	item := domain.Node{ID: domain.MustID("nod")}
	if input.ListenIP == "" {
		input.ListenIP = "0.0.0.0"
	}
	if input.KernelType == "" {
		input.KernelType = "singbox"
	}
	if len(input.Config) == 0 {
		input.Config = json.RawMessage(`{}`)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Node{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `
		INSERT INTO nodes(id,machine_id,route_policy_id,name,protocol,listen_ip,server_port,kernel_type,config)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING agent_id,status,current_revision,applied_revision,last_report_at,last_error,created_at,updated_at`,
		item.ID, input.MachineID, input.RoutePolicyID, strings.TrimSpace(input.Name), strings.ToLower(input.Protocol),
		input.ListenIP, input.ServerPort, input.KernelType, input.Config,
	).Scan(&item.AgentID, &item.Status, &item.CurrentRevision, &item.AppliedRevision, &item.LastReport,
		&item.LastError, &item.CreatedAt, &item.UpdatedAt)
	item.MachineID = input.MachineID
	item.RoutePolicyID = input.RoutePolicyID
	item.Name = strings.TrimSpace(input.Name)
	item.Protocol = strings.ToLower(input.Protocol)
	item.ListenIP = input.ListenIP
	item.ServerPort = input.ServerPort
	item.KernelType = input.KernelType
	item.Config = input.Config
	if err != nil {
		return domain.Node{}, mapError(err)
	}
	item.Endpoints, err = replaceNodeEndpoints(ctx, tx, item.ID, input.Endpoints)
	if err != nil {
		return domain.Node{}, err
	}
	return item, tx.Commit(ctx)
}

func (s *Store) UpdateNode(ctx context.Context, nodeID string, input domain.NodeUpdate) (domain.Node, error) {
	item, err := s.GetNode(ctx, nodeID)
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
		item.Status = strings.ToLower(strings.TrimSpace(*input.Status))
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Node{}, err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `
		UPDATE nodes SET machine_id=$2,route_policy_id=$3,name=$4,protocol=$5,listen_ip=$6,
		       server_port=$7,kernel_type=$8,config=$9,
		       status=CASE WHEN $10::text IS NOT NULL THEN $10 WHEN status='disabled' THEN status ELSE 'draft' END,updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, nodeID, item.MachineID, item.RoutePolicyID, item.Name,
		item.Protocol, item.ListenIP, item.ServerPort, item.KernelType, item.Config, input.Status)
	if err != nil {
		return domain.Node{}, mapError(err)
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
			VALUES($1,$2,'machine.nodes.replace',$3,jsonb_build_object('node_id',$4))`,
			originalMachineID, nodeID, item.CurrentRevision, item.AgentID); err != nil {
			return domain.Node{}, err
		}
		if originalMachineID != item.MachineID {
			if _, err := tx.Exec(ctx, `INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
				VALUES($1,$2,'machine.nodes.replace',$3,jsonb_build_object('node_id',$4))`,
				item.MachineID, nodeID, item.CurrentRevision, item.AgentID); err != nil {
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
	var item domain.Node
	err = tx.QueryRow(ctx, `
		UPDATE nodes SET current_revision=current_revision+1,status='published',updated_at=now()
		WHERE id=$1 AND status <> 'archived'
		RETURNING id,agent_id,machine_id,route_policy_id,name,protocol,listen_ip,server_port,kernel_type,
		          config,status,current_revision,applied_revision,last_report_at,last_error,created_at,updated_at`, nodeID,
	).Scan(&item.ID, &item.AgentID, &item.MachineID, &item.RoutePolicyID, &item.Name, &item.Protocol,
		&item.ListenIP, &item.ServerPort, &item.KernelType, &item.Config, &item.Status, &item.CurrentRevision,
		&item.AppliedRevision, &item.LastReport, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Node{}, ErrNotFound
	}
	if err != nil {
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
		       (SELECT count(*) FROM access_group_nodes gn WHERE gn.access_group_id=g.id),
		       (SELECT count(*) FROM users u LEFT JOIN plans p ON p.id=u.plan_id
		        WHERE COALESCE(u.access_group_override_id,p.access_group_id)=g.id AND u.status <> 'archived'),
		       ARRAY(SELECT gn.node_id FROM access_group_nodes gn WHERE gn.access_group_id=g.id ORDER BY gn.node_id),
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
	item := domain.Plan{ID: domain.MustID("pln"), ResetStrategy: "calendar_month"}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO plans(id,access_group_id,name,traffic_limit_bytes,speed_limit_mbps,device_limit,default_valid_days,notes)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING name,status,traffic_limit_bytes,speed_limit_mbps,device_limit,reset_strategy,
		          default_valid_days,notes,created_at,updated_at`,
		item.ID, input.AccessGroupID, strings.TrimSpace(input.Name), input.TrafficLimitBytes,
		input.SpeedLimitMbps, input.DeviceLimit, input.DefaultValidDays, strings.TrimSpace(input.Notes),
	).Scan(&item.Name, &item.Status, &item.TrafficLimitBytes, &item.SpeedLimitMbps, &item.DeviceLimit,
		&item.ResetStrategy, &item.DefaultValidDays, &item.Notes, &item.CreatedAt, &item.UpdatedAt)
	item.AccessGroupID = input.AccessGroupID
	return item, mapError(err)
}

func (s *Store) UpdatePlan(ctx context.Context, planID string, input domain.PlanUpdate) (domain.Plan, error) {
	command, err := s.pool.Exec(ctx, `UPDATE plans SET
		access_group_id=COALESCE(NULLIF(BTRIM($2),''),access_group_id),name=COALESCE(NULLIF(BTRIM($3),''),name),
		traffic_limit_bytes=COALESCE($4,traffic_limit_bytes),speed_limit_mbps=COALESCE($5,speed_limit_mbps),
		device_limit=COALESCE($6,device_limit),default_valid_days=COALESCE($7,default_valid_days),
		notes=COALESCE($8,notes),status=COALESCE($9,status),updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, planID, input.AccessGroupID, input.Name, input.TrafficLimitBytes,
		input.SpeedLimitMbps, input.DeviceLimit, input.DefaultValidDays, input.Notes, input.Status)
	if err != nil {
		return domain.Plan{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.Plan{}, ErrNotFound
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
	rows, err := s.pool.Query(ctx, `
		SELECT u.id,u.agent_id,u.role,u.plan_id,p.name,u.access_group_override_id,u.name,u.email,u.uuid,
		       u.subscription_token_prefix,(COALESCE(u.subscription_token_plain,'') <> ''),u.status,
		       u.traffic_limit_override_bytes,u.speed_limit_override_mbps,
		       u.device_limit_override,u.traffic_used_bytes,u.traffic_reset_at,u.expires_at,u.notes,u.created_at,u.updated_at
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
			&item.SubscriptionAvailable, &item.Status, &item.TrafficLimitOverrideBytes, &item.SpeedLimitOverrideMbps,
			&item.DeviceLimitOverride, &item.TrafficUsedBytes, &item.TrafficResetAt, &item.ExpiresAt,
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
		                  device_limit_override,expires_at,notes,traffic_reset_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,date_trunc('month',now()) + interval '1 month')
		RETURNING agent_id,role,plan_id,access_group_override_id,name,email,subscription_token_prefix,status,
		          traffic_limit_override_bytes,speed_limit_override_mbps,device_limit_override,traffic_used_bytes,
		          traffic_reset_at,expires_at,notes,created_at,updated_at`,
		item.ID, input.Role, input.PlanID, input.AccessGroupOverrideID, strings.TrimSpace(input.Name), input.Email,
		uuid, hash, auth.Prefix(plain, 12), plain, input.TrafficLimitOverrideBytes, input.SpeedLimitOverrideMbps,
		input.DeviceLimitOverride, input.ExpiresAt, strings.TrimSpace(input.Notes),
	).Scan(&item.AgentID, &item.Role, &item.PlanID, &item.AccessGroupOverrideID, &item.Name, &item.Email,
		&item.SubscriptionTokenPrefix, &item.Status, &item.TrafficLimitOverrideBytes, &item.SpeedLimitOverrideMbps,
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
	var adminLinked bool
	if err := s.pool.QueryRow(ctx, `SELECT admin_id IS NOT NULL FROM users
		WHERE id=$1 AND status <> 'archived'`, userID).Scan(&adminLinked); errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	} else if err != nil {
		return domain.User{}, err
	}
	if adminLinked && ((input.Role != nil && *input.Role != "admin") || input.Status != nil) {
		return domain.User{}, ErrConflict
	}
	if !adminLinked && input.Role != nil && *input.Role == "admin" {
		return domain.User{}, ErrConflict
	}
	command, err := s.pool.Exec(ctx, `UPDATE users SET
		role=COALESCE($2,role),uuid=COALESCE(NULLIF(BTRIM($3),''),uuid),
		plan_id=CASE WHEN $4::text IS NULL THEN plan_id ELSE NULLIF(BTRIM($4),'') END,
		access_group_override_id=CASE WHEN $5::text IS NULL THEN access_group_override_id ELSE NULLIF(BTRIM($5),'') END,
		name=COALESCE(NULLIF(BTRIM($6),''),name),email=CASE WHEN $7::text IS NULL THEN email ELSE NULLIF(BTRIM($7),'') END,
		expires_at=CASE WHEN $8::text IS NULL THEN expires_at ELSE NULLIF(BTRIM($8),'')::timestamptz END,
		notes=COALESCE($9,notes),status=COALESCE($10,status),updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, userID, input.Role, input.UUID, input.PlanID, input.AccessGroupOverrideID,
		input.Name, input.Email, input.ExpiresAt, input.Notes, input.Status)
	if err != nil {
		return domain.User{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.User{}, ErrNotFound
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
		SELECT r.id,r.name,r.status,r.current_revision,r.notes,
		       (SELECT count(*) FROM nodes n WHERE n.route_policy_id=r.id AND n.status <> 'archived'),
		       r.created_at,r.updated_at
		FROM route_policies r WHERE r.status <> 'archived' ORDER BY r.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.RoutePolicy, 0)
	for rows.Next() {
		var item domain.RoutePolicy
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.CurrentRevision, &item.Notes,
			&item.NodeCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateRoutePolicy(ctx context.Context, name, notes string) (domain.RoutePolicy, error) {
	item := domain.RoutePolicy{ID: domain.MustID("rte")}
	err := s.pool.QueryRow(ctx, `INSERT INTO route_policies(id,name,notes) VALUES($1,$2,$3)
		RETURNING name,status,current_revision,notes,created_at,updated_at`,
		item.ID, strings.TrimSpace(name), strings.TrimSpace(notes),
	).Scan(&item.Name, &item.Status, &item.CurrentRevision, &item.Notes, &item.CreatedAt, &item.UpdatedAt)
	return item, mapError(err)
}

func (s *Store) UpdateRoutePolicy(ctx context.Context, routeID string, input domain.RoutePolicyUpdate) (domain.RoutePolicy, error) {
	command, err := s.pool.Exec(ctx, `UPDATE route_policies SET
		name=COALESCE(NULLIF(BTRIM($2),''),name),notes=COALESCE($3,notes),status=COALESCE($4,status),updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, routeID, input.Name, input.Notes, input.Status)
	if err != nil {
		return domain.RoutePolicy{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.RoutePolicy{}, ErrNotFound
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

func (s *Store) CreateOutbound(ctx context.Context, input domain.Outbound) (domain.Outbound, error) {
	input.ID = domain.MustID("out")
	if len(input.Settings) == 0 {
		input.Settings = json.RawMessage(`{}`)
	}
	if len(input.KernelSupport) == 0 {
		input.KernelSupport = []string{"singbox", "xray"}
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO outbounds(id,name,tag,protocol,settings,proxy_tag,kernel_support)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING status,created_at,updated_at`,
		input.ID, strings.TrimSpace(input.Name), strings.TrimSpace(input.Tag), strings.ToLower(input.Protocol),
		input.Settings, strings.TrimSpace(input.ProxyTag), input.KernelSupport,
	).Scan(&input.Status, &input.CreatedAt, &input.UpdatedAt)
	return input, mapError(err)
}

func (s *Store) UpdateOutbound(ctx context.Context, outboundID string, input domain.OutboundUpdate) (domain.Outbound, error) {
	command, err := s.pool.Exec(ctx, `UPDATE outbounds SET
		name=COALESCE(NULLIF(BTRIM($2),''),name),tag=COALESCE(NULLIF(BTRIM($3),''),tag),
		protocol=COALESCE($4,protocol),settings=COALESCE($5,settings),proxy_tag=COALESCE($6,proxy_tag),
		kernel_support=COALESCE($7,kernel_support),status=COALESCE($8,status),updated_at=now()
		WHERE id=$1 AND status <> 'archived'`, outboundID, input.Name, input.Tag, input.Protocol,
		input.Settings, input.ProxyTag, input.KernelSupport, input.Status)
	if err != nil {
		return domain.Outbound{}, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return domain.Outbound{}, ErrNotFound
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

func (s *Store) NotifyAllPublishedNodes(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO control_changes(machine_id,node_id,event_type,revision,payload)
		SELECT machine_id,id,'node.members.replace',current_revision,jsonb_build_object('node_id',agent_id)
		FROM nodes WHERE status='published'`)
	return err
}

package store

import (
	"context"
	"errors"
	"time"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
	"golang.org/x/mod/semver"
)

type upgradeMachine struct {
	ID, Name, Version, TaskID, TargetVersion, TaskStatus string
}

// Caller holds the machine row lock until all requested upgrades are durable.
const upgradeMachineSelect = `SELECT id,name,agent_version,COALESCE(agent_upgrade_task_id,''),agent_upgrade_target_version,agent_upgrade_status
	FROM machines WHERE status NOT IN ('disabled','archived')`

const queueUpgradeSQL = `UPDATE machines SET agent_upgrade_task_id=$2,
	agent_upgrade_requested_at=now(),agent_upgrade_dispatched_at=NULL,
	agent_upgrade_target_version=$3,agent_upgrade_status='queued',
	agent_upgrade_acknowledged_at=NULL,agent_upgrade_completed_at=NULL,
	agent_upgrade_failed_at=NULL,agent_upgrade_error='',updated_at=now()
	WHERE id=$1`

const newerAgentMessage = "已安装版本高于批次目标，已停止升级"

func installedAgentIsNewer(current, target string) bool {
	current = "v" + normalizeAgentVersionForComparison(current)
	target = "v" + normalizeAgentVersionForComparison(target)
	return semver.IsValid(current) && semver.IsValid(target) && semver.Compare(current, target) > 0
}

func planBatchItem(batch *pgx.Batch, m upgradeMachine, target string) (task, state, message string) {
	if installedAgentIsNewer(m.Version, target) {
		return "", "failed", newerAgentMessage
	}
	switch m.TaskStatus {
	case "queued", "dispatched", "acknowledged":
		if sameAgentVersion(m.TargetVersion, target) {
			return m.TaskID, m.TaskStatus, ""
		}
		return "", "failed", "其他版本有升级任务正在执行"
	}
	if sameAgentVersion(m.Version, target) {
		return "", "already_current", ""
	}
	task = domain.MustID("upg")
	batch.Queue(queueUpgradeSQL, m.ID, task, target)
	return task, "queued", ""
}

func (s *Store) CreateAgentUpgradeBatch(ctx context.Context, target string) (*domain.AgentUpgradeBatch, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, upgradeMachineSelect+" ORDER BY id FOR UPDATE")
	if err != nil {
		return nil, err
	}
	machines, err := pgx.CollectRows(rows, pgx.RowToStructByPos[upgradeMachine])
	if err != nil {
		return nil, err
	}
	id := domain.MustID("batch")
	if _, err := tx.Exec(ctx, `INSERT INTO agent_upgrade_batches(id,target_version) VALUES ($1,$2)`, id, target); err != nil {
		return nil, err
	}
	var batch pgx.Batch
	for _, m := range machines {
		task, state, message := planBatchItem(&batch, m, target)
		batch.Queue(`INSERT INTO agent_upgrade_batch_items(batch_id,machine_id,machine_name,agent_version,task_id,status,error)
			VALUES($1,$2,$3,$4,$5,$6,$7)`, id, m.ID, m.Name, m.Version, task, state, message)
	}
	if err := tx.SendBatch(ctx, &batch).Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.AgentUpgradeBatch(ctx, id)
}

func (s *Store) RetryAgentUpgradeBatchItem(ctx context.Context, id, machineID, latest string) (*domain.AgentUpgradeBatch, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var m upgradeMachine
	err = tx.QueryRow(ctx, upgradeMachineSelect+" AND id=$1 FOR UPDATE", machineID).Scan(&m.ID, &m.Name, &m.Version, &m.TaskID, &m.TargetVersion, &m.TaskStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var status, target string
	err = tx.QueryRow(ctx, `SELECT i.status,b.target_version FROM agent_upgrade_batch_items i JOIN agent_upgrade_batches b ON b.id=i.batch_id
		WHERE i.batch_id=$1 AND i.machine_id=$2 FOR UPDATE OF i`, id, machineID).Scan(&status, &target)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "failed" && status != "timed_out" {
		return nil, ErrConflict
	}
	if target != latest {
		return nil, ErrAgentReleaseChanged
	}
	var batch pgx.Batch
	task, state, message := planBatchItem(&batch, m, target)
	if message == newerAgentMessage {
		return nil, ErrAgentAlreadyNewer
	}
	if state == "failed" {
		return nil, ErrAgentUpgradePending
	}
	batch.Queue(`UPDATE agent_upgrade_batch_items SET task_id=$3,status=$4,error=$5,agent_version=$6,updated_at=now()
		WHERE batch_id=$1 AND machine_id=$2`, id, machineID, task, state, message, m.Version)
	if err := tx.SendBatch(ctx, &batch).Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.AgentUpgradeBatch(ctx, id)
}

// Empty id selects the latest batch. A single statement reads one consistent
// snapshot even while heartbeats or other administrators change a command.
func (s *Store) AgentUpgradeBatch(ctx context.Context, id string) (*domain.AgentUpgradeBatch, error) {
	threshold := int64(s.machineOfflineThreshold(ctx) / time.Second)
	rows, err := s.pool.Query(ctx, `WITH selected AS (
		SELECT id,target_version,created_at FROM agent_upgrade_batches WHERE $1='' OR id=$1 ORDER BY created_at DESC,id DESC LIMIT 1
	) SELECT b.id,b.target_version,b.created_at,COALESCE(i.machine_id,''),COALESCE(i.machine_name,''),COALESCE(i.agent_version,''),
		COALESCE(i.task_id,''),CASE WHEN i.status='queued' AND (m.status IS DISTINCT FROM 'online' OR
		m.last_heartbeat_at IS NULL OR m.last_heartbeat_at<now()-$2*interval '1 second') THEN 'waiting_online' ELSE COALESCE(i.status,'') END,
		COALESCE(i.error,''),COALESCE(i.updated_at,b.created_at)
	FROM selected b LEFT JOIN agent_upgrade_batch_items i ON i.batch_id=b.id LEFT JOIN machines m ON m.id=i.machine_id
	ORDER BY i.machine_name,i.machine_id`, id, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := domain.AgentUpgradeBatch{Items: []domain.AgentUpgradeBatchItem{}}
	for rows.Next() {
		var i domain.AgentUpgradeBatchItem
		if err := rows.Scan(&result.ID, &result.TargetVersion, &result.CreatedAt, &i.MachineID, &i.MachineName, &i.AgentVersion, &i.TaskID, &i.Status, &i.Error, &i.UpdatedAt); err != nil {
			return nil, err
		}
		if i.MachineID != "" {
			result.Items = append(result.Items, i)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result.ID == "" {
		if id != "" {
			return nil, ErrNotFound
		}
		return nil, nil
	}
	return &result, nil
}

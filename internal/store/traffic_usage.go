package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func trafficMonthBounds(now time.Time, location *time.Location) (string, string, time.Time) {
	if location == nil {
		location = loadTrafficLocation("")
	}
	localNow := now.In(location)
	monthStart := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, location)
	nextMonth := monthStart.AddDate(0, 1, 0)
	return monthStart.Format(time.DateOnly), nextMonth.Format(time.DateOnly), nextMonth
}

func (s *Store) ReconcileTrafficUsage(ctx context.Context) (int64, error) {
	location := loadTrafficLocation(s.SettingString(ctx, "site", "timezone", "Asia/Shanghai"))
	monthStart, nextMonth, nextReset := trafficMonthBounds(time.Now(), location)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	// Telemetry locks the same users in agent_id order before adding detail or
	// counters. Lock first, then aggregate in a fresh statement snapshot so a
	// reconciliation cannot overwrite a concurrently committed traffic report.
	if err := lockAllTrafficUsers(ctx, tx); err != nil {
		return 0, err
	}

	rows, err := tx.Query(ctx, `WITH combined AS (
		SELECT user_id,upload_bytes,download_bytes FROM traffic_daily
		WHERE day >= $1::date AND day < $2::date
		UNION ALL
		SELECT user_id,upload_bytes,download_bytes FROM imported_user_traffic_daily
		WHERE day >= $1::date AND day < $2::date
	), monthly AS (
		SELECT user_id,sum(upload_bytes+download_bytes)::bigint AS used_bytes
		FROM combined GROUP BY user_id
	), expected AS (
		SELECT u.id,COALESCE(m.used_bytes,0)::bigint AS used_bytes,
		       COALESCE(u.traffic_limit_override_bytes,p.traffic_limit_bytes,0) AS quota,
		       u.traffic_used_bytes AS old_used_bytes
		FROM users u
		JOIN plans p ON p.id=u.plan_id AND p.reset_strategy='calendar_month'
		LEFT JOIN monthly m ON m.user_id=u.id
		WHERE u.status <> 'archived'
	)
	UPDATE users u SET traffic_used_bytes=e.used_bytes,traffic_reset_at=$3
	FROM expected e
	WHERE u.id=e.id AND (u.traffic_used_bytes IS DISTINCT FROM e.used_bytes OR u.traffic_reset_at IS DISTINCT FROM $3)
	RETURNING u.id,(e.quota>0 AND e.old_used_bytes>=e.quota) IS DISTINCT FROM (e.quota>0 AND e.used_bytes>=e.quota)`,
		monthStart, nextMonth, nextReset)
	if err != nil {
		return 0, err
	}
	var count int64
	var changedUsers []string
	for rows.Next() {
		var id string
		var authorizationChanged bool
		if err := rows.Scan(&id, &authorizationChanged); err != nil {
			rows.Close()
			return 0, err
		}
		count++
		if authorizationChanged {
			changedUsers = append(changedUsers, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, id := range changedUsers {
		if err := notifyUserPublishedNodesTx(ctx, tx, id); err != nil {
			return 0, err
		}
	}

	withoutResetTag, err := tx.Exec(ctx, `UPDATE users u SET traffic_reset_at=NULL
		WHERE u.status <> 'archived' AND u.traffic_reset_at IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM plans p WHERE p.id=u.plan_id AND p.reset_strategy='calendar_month'
		  )`)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count + withoutResetTag.RowsAffected(), nil
}

func lockAllTrafficUsers(ctx context.Context, q resourceQuerier) error {
	rows, err := q.Query(ctx, `SELECT id FROM users WHERE status <> 'archived' ORDER BY agent_id FOR UPDATE`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
	}
	return rows.Err()
}

// recordUserTraffic runs after all users in the batch have been locked. Quota
// affects access, never acceptance of a final report from a previously allowed
// user; traffic accounting therefore intentionally does not use the quota CTE.
func recordUserTraffic(ctx context.Context, tx pgx.Tx, nodeID, machineID string, agentID int64, sampledAt time.Time, location *time.Location, traffic [2]int64) error {
	var userID string
	var before, quota int64
	err := tx.QueryRow(ctx, trafficAccountableUsersCTE+`
		SELECT u.id,u.traffic_used_bytes,u.traffic_limit_bytes FROM nodes n
		JOIN access_group_nodes gn ON gn.node_id=n.id
		JOIN traffic_accountable_users u ON u.agent_id=$3
		WHERE n.id=$1 AND n.machine_id=$2 AND gn.access_group_id=u.access_group_id`, nodeID, machineID, agentID).
		Scan(&userID, &before, &quota)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO traffic_daily(day,user_id,node_id,upload_bytes,download_bytes)
		VALUES($1::date,$2,$3,$4,$5)
		ON CONFLICT(day,user_id,node_id) DO UPDATE SET
		  upload_bytes=traffic_daily.upload_bytes+EXCLUDED.upload_bytes,
		  download_bytes=traffic_daily.download_bytes+EXCLUDED.download_bytes`,
		telemetryDay(sampledAt, location), userID, nodeID, traffic[0], traffic[1]); err != nil {
		return err
	}
	start, end, reset := trafficMonthBounds(time.Now(), location)
	var after int64
	err = tx.QueryRow(ctx, `WITH effective AS (
		SELECT u.id,p.reset_strategy FROM users u LEFT JOIN plans p ON p.id=u.plan_id WHERE u.id=$1
	), monthly AS (
		SELECT COALESCE(sum(upload_bytes+download_bytes),0)::bigint AS used_bytes FROM (
			SELECT upload_bytes,download_bytes FROM traffic_daily WHERE user_id=$1 AND day >= $4::date AND day < $5::date
			UNION ALL
			SELECT upload_bytes,download_bytes FROM imported_user_traffic_daily WHERE user_id=$1 AND day >= $4::date AND day < $5::date
		) combined
	)
	UPDATE users u SET traffic_used_bytes=CASE WHEN e.reset_strategy='calendar_month' THEN m.used_bytes ELSE u.traffic_used_bytes+$2+$3 END,
		traffic_reset_at=CASE WHEN e.reset_strategy='calendar_month' THEN $6::timestamptz ELSE NULL END
	FROM effective e,monthly m WHERE u.id=e.id RETURNING u.traffic_used_bytes`, userID, traffic[0], traffic[1], start, end, reset).
		Scan(&after)
	if err != nil {
		return err
	}
	if quota > 0 && (before >= quota) != (after >= quota) {
		return notifyUserPublishedNodesTx(ctx, tx, userID)
	}
	return nil
}

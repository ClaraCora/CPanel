package store

import (
	"context"
	"time"
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

	monthlyTag, err := tx.Exec(ctx, `WITH combined AS (
		SELECT user_id,upload_bytes,download_bytes FROM traffic_daily
		WHERE day >= $1::date AND day < $2::date
		UNION ALL
		SELECT user_id,upload_bytes,download_bytes FROM imported_user_traffic_daily
		WHERE day >= $1::date AND day < $2::date
	), monthly AS (
		SELECT user_id,sum(upload_bytes+download_bytes)::bigint AS used_bytes
		FROM combined GROUP BY user_id
	), expected AS (
		SELECT u.id,COALESCE(m.used_bytes,0)::bigint AS used_bytes
		FROM users u
		JOIN plans p ON p.id=u.plan_id AND p.reset_strategy='calendar_month'
		LEFT JOIN monthly m ON m.user_id=u.id
		WHERE u.status <> 'archived'
	)
	UPDATE users u SET traffic_used_bytes=e.used_bytes,traffic_reset_at=$3
	FROM expected e
	WHERE u.id=e.id AND (u.traffic_used_bytes IS DISTINCT FROM e.used_bytes OR u.traffic_reset_at IS DISTINCT FROM $3)`,
		monthStart, nextMonth, nextReset)
	if err != nil {
		return 0, err
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
	return monthlyTag.RowsAffected() + withoutResetTag.RowsAffected(), nil
}

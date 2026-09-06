package store

import (
	"context"
	"fmt"

	"cpanel/internal/domain"
)

func (s *Store) HistoricalData(ctx context.Context) (domain.HistoricalData, error) {
	result := domain.HistoricalData{
		Retention: domain.HistoricalRetention{
			DevicesDays: s.SettingInt(ctx, "retention", "devices_days", 30, 1),
			TrafficDays: s.SettingInt(ctx, "retention", "traffic_days", 90, 1),
		},
	}
	var err error
	result.Traffic, err = s.listDailyTraffic(ctx, result.Retention.TrafficDays)
	if err != nil {
		return result, err
	}
	result.MachineMetrics, err = s.listMetricSamples(ctx, `
		SELECT mm.machine_id,m.name,mm.sampled_at,mm.metrics
		FROM machine_metrics mm JOIN machines m ON m.id=mm.machine_id
		ORDER BY mm.sampled_at DESC LIMIT 300`)
	if err != nil {
		return result, err
	}
	result.NodeMetrics, err = s.listMetricSamples(ctx, `
		SELECT nm.node_id,n.name,nm.sampled_at,nm.metrics
		FROM node_metrics nm JOIN nodes n ON n.id=nm.node_id
		ORDER BY nm.sampled_at DESC LIMIT 300`)
	if err != nil {
		return result, err
	}
	result.Devices, err = s.listDeviceHistory(ctx, result.Retention.DevicesDays)
	return result, err
}

func (s *Store) listDailyTraffic(ctx context.Context, days int) ([]domain.DailyTraffic, error) {
	rows, err := s.pool.Query(ctx, `WITH combined AS (
		SELECT day,upload_bytes,download_bytes FROM traffic_daily
		UNION ALL
		SELECT day,upload_bytes,download_bytes FROM imported_user_traffic_daily
	)
	SELECT day,sum(upload_bytes),sum(download_bytes) FROM combined
	WHERE day >= current_date - ($1::int - 1)
	GROUP BY day ORDER BY day DESC`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.DailyTraffic, 0)
	for rows.Next() {
		var item domain.DailyTraffic
		if err := rows.Scan(&item.Day, &item.UploadBytes, &item.DownloadBytes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListTrafficPage(ctx context.Context, days, page, pageSize int) ([]domain.DailyTraffic, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM (SELECT day FROM traffic_daily WHERE day >= current_date - ($1::int - 1) UNION SELECT day FROM imported_user_traffic_daily WHERE day >= current_date - ($1::int - 1)) days`, days).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `WITH combined AS (SELECT day,upload_bytes,download_bytes FROM traffic_daily UNION ALL SELECT day,upload_bytes,download_bytes FROM imported_user_traffic_daily)
		SELECT day,sum(upload_bytes),sum(download_bytes) FROM combined WHERE day >= current_date - ($1::int - 1) GROUP BY day ORDER BY day DESC LIMIT $2 OFFSET $3`, days, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.DailyTraffic, 0)
	for rows.Next() {
		var item domain.DailyTraffic
		if err := rows.Scan(&item.Day, &item.UploadBytes, &item.DownloadBytes); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) ListMetricSamplesPage(ctx context.Context, kind string, page, pageSize int) ([]domain.MetricSample, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	resourceTable, joinTable := "machine_metrics", "machines"
	idColumn := "machine_id"
	if kind == "nodes" {
		resourceTable, joinTable, idColumn = "node_metrics", "nodes", "node_id"
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM `+resourceTable).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT m.`+idColumn+`,r.name,m.sampled_at,m.metrics FROM `+resourceTable+` m JOIN `+joinTable+` r ON r.id=m.`+idColumn+` ORDER BY m.sampled_at DESC LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.MetricSample, 0)
	for rows.Next() {
		var item domain.MetricSample
		if err := rows.Scan(&item.ResourceID, &item.ResourceName, &item.SampledAt, &item.Metrics); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) ListDeviceHistoryPage(ctx context.Context, days, page, pageSize int) ([]domain.DeviceHistory, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM user_devices WHERE last_seen_at >= now() - ($1::int * interval '1 day')`, days).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT d.id,d.user_id,u.name,d.node_id,n.name,host(d.ip_address),d.first_seen_at,d.last_seen_at,d.online FROM user_devices d JOIN users u ON u.id=d.user_id JOIN nodes n ON n.id=d.node_id WHERE d.last_seen_at >= now() - ($1::int * interval '1 day') ORDER BY d.last_seen_at DESC LIMIT $2 OFFSET $3`, days, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.DeviceHistory, 0)
	for rows.Next() {
		var item domain.DeviceHistory
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserName, &item.NodeID, &item.NodeName, &item.IPAddress, &item.FirstSeen, &item.LastSeen, &item.Online); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) listMetricSamples(ctx context.Context, query string) ([]domain.MetricSample, error) {
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.MetricSample, 0)
	for rows.Next() {
		var item domain.MetricSample
		if err := rows.Scan(&item.ResourceID, &item.ResourceName, &item.SampledAt, &item.Metrics); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) listDeviceHistory(ctx context.Context, days int) ([]domain.DeviceHistory, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id,d.user_id,u.name,d.node_id,n.name,host(d.ip_address),d.first_seen_at,d.last_seen_at,d.online
		FROM user_devices d JOIN users u ON u.id=d.user_id JOIN nodes n ON n.id=d.node_id
		WHERE d.last_seen_at >= now() - ($1::int * interval '1 day')
		ORDER BY d.last_seen_at DESC LIMIT 300`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.DeviceHistory, 0)
	for rows.Next() {
		var item domain.DeviceHistory
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserName, &item.NodeID, &item.NodeName,
			&item.IPAddress, &item.FirstSeen, &item.LastSeen, &item.Online); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) PruneHistoricalData(ctx context.Context) error {
	devicesDays := s.SettingInt(ctx, "retention", "devices_days", 30, 1)
	trafficDays := s.SettingInt(ctx, "retention", "traffic_days", 90, 1)
	auditDays := s.SettingInt(ctx, "retention", "audit_days", 180, 1)
	subscriptionAccessDays := s.SettingInt(ctx, "retention", "subscription_access_days", 30, 1)
	queries := []struct {
		query string
		days  int
	}{
		{`DELETE FROM user_devices WHERE last_seen_at < now() - ($1::int * interval '1 day')`, devicesDays},
		{`DELETE FROM traffic_daily WHERE day < current_date - $1::int`, trafficDays},
		{`DELETE FROM imported_node_traffic_daily WHERE day < current_date - $1::int`, trafficDays},
		{`DELETE FROM imported_user_traffic_daily WHERE day < current_date - $1::int`, trafficDays},
		{`DELETE FROM audit_events WHERE created_at < now() - ($1::int * interval '1 day')`, auditDays},
		{`DELETE FROM subscription_access_events WHERE created_at < now() - ($1::int * interval '1 day')`, subscriptionAccessDays},
	}
	for _, item := range queries {
		if _, err := s.pool.Exec(ctx, item.query, item.days); err != nil {
			return fmt.Errorf("prune historical data: %w", err)
		}
	}
	return nil
}

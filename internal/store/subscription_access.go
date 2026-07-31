package store

import (
	"context"

	"cpanel/internal/domain"
)

func (s *Store) RecordSubscriptionAccess(ctx context.Context, userID *string, userName, ipAddress, userAgent, outcome string, statusCode int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subscription_access_events(id,user_id,user_name,ip_address,user_agent,outcome,status_code)
		VALUES($1,$2,$3,NULLIF($4,'')::inet,$5,$6,$7)`,
		domain.MustID("sac"), userID, userName, ipAddress, userAgent, outcome, statusCode)
	return err
}

func (s *Store) ListSubscriptionAccess(ctx context.Context, limit int) ([]domain.SubscriptionAccessEvent, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id,user_id,user_name,COALESCE(host(ip_address),''),user_agent,outcome,status_code,created_at
		FROM subscription_access_events
		ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.SubscriptionAccessEvent, 0)
	for rows.Next() {
		var item domain.SubscriptionAccessEvent
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserName, &item.IPAddress, &item.UserAgent, &item.Outcome, &item.StatusCode, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

package store

import (
	"context"
	"strings"

	"cpanel/internal/domain"
)

func (s *Store) RecordSubscriptionAccess(ctx context.Context, userID *string, userName, ipAddress, userAgent, outcome string, statusCode int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subscription_access_events(id,user_id,user_name,ip_address,user_agent,outcome,status_code)
		VALUES($1,$2,$3,NULLIF($4,'')::inet,$5,$6,$7)`,
		domain.MustID("sac"), userID, userName, ipAddress, userAgent, outcome, statusCode)
	return err
}

func (s *Store) ListSubscriptionAccessPage(ctx context.Context, page, pageSize int, query, sortField, order string) ([]domain.SubscriptionAccessEvent, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	orderSQL := "DESC"
	if strings.EqualFold(order, "asc") {
		orderSQL = "ASC"
	}
	sortSQL := "created_at"
	if sortField == "status_code" {
		sortSQL = "status_code"
	}
	pattern := "%" + strings.TrimSpace(query) + "%"
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM subscription_access_events WHERE ($1='' OR user_name ILIKE $2 OR host(ip_address) ILIKE $2 OR user_agent ILIKE $2 OR outcome ILIKE $2)`, query, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,user_id,user_name,COALESCE(host(ip_address),''),user_agent,outcome,status_code,created_at
		FROM subscription_access_events WHERE ($1='' OR user_name ILIKE $2 OR host(ip_address) ILIKE $2 OR user_agent ILIKE $2 OR outcome ILIKE $2)
		ORDER BY `+sortSQL+` `+orderSQL+`, id DESC LIMIT $3 OFFSET $4`, query, pattern, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.SubscriptionAccessEvent, 0)
	for rows.Next() {
		var item domain.SubscriptionAccessEvent
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserName, &item.IPAddress, &item.UserAgent, &item.Outcome, &item.StatusCode, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
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

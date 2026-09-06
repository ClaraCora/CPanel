package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ListUserAccessIPs(ctx context.Context) ([]domain.UserAccessIPAccount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id,u.name,u.role,u.status,host(a.ip_address),a.first_seen_at,a.last_seen_at,
		       a.last_node_id,a.last_node_name,a.location_scope,a.location_country_code,
		       a.location_country,a.location_province,a.location_city,a.location_isp,a.location_updated_at
		FROM users u
		JOIN user_access_ips a ON a.user_id=u.id
		WHERE u.status <> 'archived'
		ORDER BY lower(u.name),u.id,a.last_seen_at DESC,host(a.ip_address) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.UserAccessIPAccount, 0)
	positions := make(map[string]int)
	for rows.Next() {
		var userID, userName, role, status string
		var address domain.UserAccessIPAddress
		var location domain.UserAccessIPLocation
		var locationUpdatedAt *time.Time
		if err := rows.Scan(&userID, &userName, &role, &status, &address.IPAddress, &address.FirstSeenAt,
			&address.LastSeenAt, &address.LastNodeID, &address.LastNodeName, &location.Scope,
			&location.CountryCode, &location.Country, &location.Province, &location.City, &location.ISP,
			&locationUpdatedAt); err != nil {
			return nil, err
		}
		if locationUpdatedAt != nil {
			location.ResolvedAt = *locationUpdatedAt
			address.Location = &location
		}
		position, exists := positions[userID]
		if !exists {
			position = len(items)
			positions[userID] = position
			items = append(items, domain.UserAccessIPAccount{
				UserID: userID, UserName: userName, Role: role, Status: status,
				LastSeenAt: address.LastSeenAt, Addresses: make([]domain.UserAccessIPAddress, 0, 10),
			})
		}
		items[position].Addresses = append(items[position].Addresses, address)
	}
	return items, rows.Err()
}

func (s *Store) ListUserAccessIPsPage(ctx context.Context, page, pageSize int, query, sortField, order, role string) ([]domain.UserAccessIPAccount, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	pattern := "%" + strings.TrimSpace(query) + "%"
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users u WHERE u.status <> 'archived' AND ($3='' OR u.role=$3) AND ($1='' OR u.id ILIKE $2 OR u.name ILIKE $2 OR u.role ILIKE $2 OR u.status ILIKE $2 OR EXISTS (SELECT 1 FROM user_access_ips x WHERE x.user_id=u.id AND host(x.ip_address) ILIKE $2))`, query, pattern, role).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `WITH selected_users AS (
		SELECT u.id,u.name,u.role,u.status FROM users u
		WHERE u.status <> 'archived' AND ($3='' OR u.role=$3) AND ($1='' OR u.id ILIKE $2 OR u.name ILIKE $2 OR u.role ILIKE $2 OR u.status ILIKE $2 OR EXISTS (SELECT 1 FROM user_access_ips x WHERE x.user_id=u.id AND host(x.ip_address) ILIKE $2))
		ORDER BY lower(u.name),u.id LIMIT $4 OFFSET $5)
		SELECT u.id,u.name,u.role,u.status,host(a.ip_address),a.first_seen_at,a.last_seen_at,a.last_node_id,a.last_node_name,
			a.location_scope,a.location_country_code,a.location_country,a.location_province,a.location_city,a.location_isp,a.location_updated_at
		FROM selected_users u JOIN user_access_ips a ON a.user_id=u.id ORDER BY lower(u.name),u.id,a.last_seen_at DESC,host(a.ip_address) DESC`, query, pattern, role, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.UserAccessIPAccount, 0)
	positions := make(map[string]int)
	for rows.Next() {
		var userID, userName, role, status string
		var address domain.UserAccessIPAddress
		var location domain.UserAccessIPLocation
		var updated *time.Time
		if err := rows.Scan(&userID, &userName, &role, &status, &address.IPAddress, &address.FirstSeenAt, &address.LastSeenAt, &address.LastNodeID, &address.LastNodeName, &location.Scope, &location.CountryCode, &location.Country, &location.Province, &location.City, &location.ISP, &updated); err != nil {
			return nil, 0, err
		}
		if updated != nil {
			location.ResolvedAt = *updated
			address.Location = &location
		}
		position, ok := positions[userID]
		if !ok {
			position = len(items)
			positions[userID] = position
			items = append(items, domain.UserAccessIPAccount{UserID: userID, UserName: userName, Role: role, Status: status, LastSeenAt: address.LastSeenAt, Addresses: make([]domain.UserAccessIPAddress, 0, 10)})
		}
		items[position].Addresses = append(items[position].Addresses, address)
	}
	return items, total, rows.Err()
}

func (s *Store) GetUserAccessIPLocation(ctx context.Context, ipAddress string) (domain.UserAccessIPLocation, bool, error) {
	var location domain.UserAccessIPLocation
	var resolvedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT location_scope,location_country_code,location_country,location_province,
		       location_city,location_isp,location_updated_at
		FROM user_access_ips
		WHERE ip_address=$1::inet
		ORDER BY location_updated_at DESC NULLS LAST
		LIMIT 1`, ipAddress).Scan(&location.Scope, &location.CountryCode, &location.Country,
		&location.Province, &location.City, &location.ISP, &resolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.UserAccessIPLocation{}, false, ErrNotFound
	}
	if err != nil {
		return domain.UserAccessIPLocation{}, false, err
	}
	if resolvedAt == nil {
		return domain.UserAccessIPLocation{}, false, nil
	}
	location.ResolvedAt = *resolvedAt
	return location, true, nil
}

func (s *Store) UpdateUserAccessIPLocation(ctx context.Context, ipAddress string, location domain.UserAccessIPLocation) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE user_access_ips SET
			location_scope=$2,location_country_code=$3,location_country=$4,
			location_province=$5,location_city=$6,location_isp=$7,location_updated_at=$8
		WHERE ip_address=$1::inet`, ipAddress, location.Scope, location.CountryCode, location.Country,
		location.Province, location.City, location.ISP, location.ResolvedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

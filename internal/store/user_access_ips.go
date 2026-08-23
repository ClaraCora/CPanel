package store

import (
	"context"

	"cpanel/internal/domain"
)

func (s *Store) ListUserAccessIPs(ctx context.Context) ([]domain.UserAccessIPAccount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id,u.name,u.role,u.status,host(a.ip_address),a.first_seen_at,a.last_seen_at,
		       a.last_node_id,a.last_node_name
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
		if err := rows.Scan(&userID, &userName, &role, &status, &address.IPAddress, &address.FirstSeenAt,
			&address.LastSeenAt, &address.LastNodeID, &address.LastNodeName); err != nil {
			return nil, err
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

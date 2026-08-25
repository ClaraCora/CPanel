package store

import (
	"context"
	"errors"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

const subscriptionEndpointQuery = `
		SELECT n.name,e.name,e.host,e.port,n.protocol,
		       CASE WHEN m.status='online' AND n.last_report_at > now() - interval '3 minutes' THEN 'online' ELSE 'offline' END,
		       u.uuid,n.config
		FROM runtime_eligible_users u
		JOIN access_group_nodes gn ON gn.access_group_id=u.access_group_id
		JOIN nodes n ON n.id=gn.node_id AND n.status='published'
		JOIN machines m ON m.id=n.machine_id AND m.status NOT IN ('disabled','archived')
		JOIN LATERAL (
			SELECT ep.name,ep.host,ep.port,ep.sort_order,ep.access_scope FROM node_endpoints ep
			WHERE ep.node_id=n.id AND ep.status='active'
			UNION ALL
			SELECT n.name,m.host,n.server_port,0,'default'
			WHERE NOT EXISTS (SELECT 1 FROM node_endpoints configured WHERE configured.node_id=n.id)
		) e ON true
		WHERE u.id=$1 AND (u.role='admin' OR e.access_scope='default')
		ORDER BY n.name,e.sort_order,e.name`

func (s *Store) SubscriptionByToken(ctx context.Context, tokenHash []byte) (domain.Subscription, error) {
	var userID string
	err := s.pool.QueryRow(ctx, runtimeEligibleUsersCTE+`
		SELECT id FROM runtime_eligible_users
		WHERE subscription_token_hash=$1`, tokenHash,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Subscription{}, ErrNotFound
	}
	if err != nil {
		return domain.Subscription{}, err
	}
	return s.SubscriptionByUserID(ctx, userID)
}

func (s *Store) SubscriptionByUserID(ctx context.Context, userID string) (domain.Subscription, error) {
	var subscription domain.Subscription
	err := s.pool.QueryRow(ctx, runtimeEligibleUsersCTE+`
		SELECT id,name,role,expires_at FROM runtime_eligible_users
		WHERE id=$1`, userID,
	).Scan(&subscription.UserID, &subscription.UserName, &subscription.Role, &subscription.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Subscription{}, ErrNotFound
	}
	if err != nil {
		return domain.Subscription{}, err
	}
	rows, err := s.pool.Query(ctx, runtimeEligibleUsersCTE+subscriptionEndpointQuery, subscription.UserID)
	if err != nil {
		return domain.Subscription{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var node domain.SubscriptionNode
		if err := rows.Scan(&node.Name, &node.EntryName, &node.Host, &node.Port, &node.Protocol, &node.Status, &node.UserUUID, &node.NodeConfig); err != nil {
			return domain.Subscription{}, err
		}
		subscription.Nodes = append(subscription.Nodes, node)
	}
	return subscription, rows.Err()
}

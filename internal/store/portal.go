package store

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) FindPortalUserByLogin(ctx context.Context, login string) (domain.PortalUserAuth, error) {
	var item domain.PortalUserAuth
	err := s.pool.QueryRow(ctx, `
		SELECT id,name,email,role,status,COALESCE(portal_password_hash,'')
		FROM users
		WHERE lower(portal_login)=lower($1)
		  AND role IN ('user','friend')
		  AND status='active'
		  AND (expires_at IS NULL OR expires_at > now())`, strings.TrimSpace(login),
	).Scan(&item.ID, &item.Name, &item.Email, &item.Role, &item.Status, &item.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PortalUserAuth{}, ErrNotFound
	}
	return item, err
}

func (s *Store) FindPortalUserByID(ctx context.Context, userID string) (domain.PortalUserAuth, error) {
	var item domain.PortalUserAuth
	err := s.pool.QueryRow(ctx, `
		SELECT id,name,email,role,status,COALESCE(portal_password_hash,'')
		FROM users
		WHERE id=$1 AND status='active' AND (expires_at IS NULL OR expires_at > now())`, userID,
	).Scan(&item.ID, &item.Name, &item.Email, &item.Role, &item.Status, &item.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PortalUserAuth{}, ErrNotFound
	}
	return item, err
}

func (s *Store) CreatePortalSession(ctx context.Context, userID string, tokenHash []byte, csrfToken, delegatedByAdminID, ipAddress, userAgent string, readOnly bool, expiresAt time.Time) (string, error) {
	id := domain.MustID("edu")
	var ip any
	if parsed := net.ParseIP(ipAddress); parsed != nil {
		ip = parsed.String()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_sessions(id,user_id,token_hash,csrf_token,delegated_by_admin_id,read_only,ip_address,user_agent,expires_at)
		VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9)`,
		id, userID, tokenHash, csrfToken, delegatedByAdminID, readOnly, ip, userAgent, expiresAt,
	)
	return id, mapError(err)
}

func (s *Store) FindPortalSession(ctx context.Context, tokenHash []byte) (domain.PortalSession, error) {
	var session domain.PortalSession
	err := s.pool.QueryRow(ctx, `
		SELECT s.id,s.csrf_token,s.expires_at,s.read_only,
		       COALESCE(s.delegated_by_admin_id,''),COALESCE(a.name,''),
		       u.id,u.name,u.email,u.role,u.status
		FROM user_sessions s
		JOIN users u ON u.id=s.user_id
		LEFT JOIN admins a ON a.id=s.delegated_by_admin_id
		WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at > now()
		  AND u.status='active' AND (u.expires_at IS NULL OR u.expires_at > now())`, tokenHash,
	).Scan(&session.ID, &session.CSRFToken, &session.ExpiresAt, &session.ReadOnly,
		&session.DelegatedByID, &session.DelegatedByName,
		&session.User.ID, &session.User.Name, &session.User.Email, &session.User.Role, &session.User.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PortalSession{}, ErrNotFound
	}
	return session, err
}

func (s *Store) RevokePortalSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `UPDATE user_sessions SET revoked_at=now() WHERE token_hash=$1`, tokenHash)
	return err
}

func (s *Store) UpdatePortalPassword(ctx context.Context, userID, currentSessionID, passwordHash string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE users SET portal_password_hash=$2,portal_enabled_at=now(),updated_at=now()
		WHERE id=$1 AND status='active'`, userID, passwordHash)
	if err != nil {
		return 0, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return 0, ErrNotFound
	}
	command, err = tx.Exec(ctx, `UPDATE user_sessions SET revoked_at=now()
		WHERE user_id=$1 AND id<>$2 AND revoked_at IS NULL`, userID, currentSessionID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

func (s *Store) CreatePortalGrant(ctx context.Context, adminID, userID string, tokenHash []byte, expiresAt time.Time) error {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status <> 'archived')`, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO user_portal_grants(id,user_id,admin_id,token_hash,expires_at)
		VALUES($1,$2,$3,$4,$5)`, domain.MustID("edg"), userID, adminID, tokenHash, expiresAt)
	return mapError(err)
}

func (s *Store) RedeemPortalGrant(ctx context.Context, tokenHash []byte) (userID, adminID string, err error) {
	err = s.pool.QueryRow(ctx, `
		UPDATE user_portal_grants SET redeemed_at=now()
		WHERE token_hash=$1 AND redeemed_at IS NULL AND expires_at > now()
		RETURNING user_id,admin_id`, tokenHash,
	).Scan(&userID, &adminID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return userID, adminID, err
}

func (s *Store) PortalDashboard(ctx context.Context, userID string) (domain.PortalDashboard, error) {
	var item domain.PortalDashboard
	err := s.pool.QueryRow(ctx, `
		SELECT u.id,u.name,u.role,u.email,p.name,u.status,u.traffic_used_bytes,
		       COALESCE(u.traffic_limit_override_bytes,p.traffic_limit_bytes,0),u.traffic_reset_at,u.expires_at,
		       COALESCE(u.speed_limit_override_mbps,p.speed_limit_mbps,0),
		       COALESCE(u.device_limit_override,p.device_limit,0)
		FROM users u LEFT JOIN plans p ON p.id=u.plan_id
		WHERE u.id=$1 AND u.status='active' AND (u.expires_at IS NULL OR u.expires_at > now())`, userID,
	).Scan(&item.ID, &item.Name, &item.Role, &item.Email, &item.PlanName, &item.Status, &item.TrafficUsedBytes,
		&item.TrafficLimitBytes, &item.TrafficResetAt, &item.ExpiresAt, &item.SpeedLimitMbps, &item.DeviceLimit)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PortalDashboard{}, ErrNotFound
	}
	return item, err
}

func (s *Store) RotateUserSubscriptionToken(ctx context.Context, userID string) (string, error) {
	plain, hash, err := auth.NewSecret("cps_", 32)
	if err != nil {
		return "", err
	}
	command, err := s.pool.Exec(ctx, `UPDATE users SET subscription_token_hash=$2,subscription_token_prefix=$3,
		subscription_token_plain=$4,updated_at=now() WHERE id=$1 AND status='active'`,
		userID, hash, auth.Prefix(plain, 12), plain)
	if err != nil {
		return "", mapError(err)
	}
	if command.RowsAffected() == 0 {
		return "", ErrNotFound
	}
	return plain, nil
}

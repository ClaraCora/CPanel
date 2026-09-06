package store

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"cpanel/internal/auth"
	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) AdminCount(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, "SELECT count(*) FROM admins").Scan(&count)
	return count, err
}

func (s *Store) EnsureAdminUsers(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id,a.email,a.name,a.status,u.id
		FROM admins a LEFT JOIN users u ON u.admin_id=a.id
		ORDER BY a.created_at`)
	if err != nil {
		return err
	}
	type candidate struct {
		id, email, name, status string
		userID                  *string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.email, &item.name, &item.status, &item.userID); err != nil {
			rows.Close()
			return err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	changed := false
	for _, item := range candidates {
		desiredStatus := "active"
		if item.status != "active" {
			desiredStatus = "paused"
		}
		if item.userID != nil {
			command, err := s.pool.Exec(ctx, `UPDATE users SET role='admin',status=$2,updated_at=now()
				WHERE admin_id=$1 AND (role<>'admin' OR status<>$2)`, item.id, desiredStatus)
			if err != nil {
				return mapError(err)
			}
			changed = changed || command.RowsAffected() > 0
			continue
		}

		uuid, err := domain.NewUUID()
		if err != nil {
			return err
		}
		plain, hash, err := auth.NewSecret("cps_", 32)
		if err != nil {
			return err
		}
		command, err := s.pool.Exec(ctx, `
			INSERT INTO users(id,admin_id,role,plan_id,name,email,uuid,subscription_token_hash,
			                  subscription_token_prefix,subscription_token_plain,status,notes,traffic_reset_at)
			SELECT $1,$2,'admin',(SELECT id FROM plans WHERE status='active' ORDER BY created_at,id LIMIT 1),
			       $3,CASE WHEN EXISTS(SELECT 1 FROM users WHERE lower(email)=lower($4)) THEN NULL ELSE lower($4) END,
			       $5,$6,$7,$8,$9,'管理员订阅账号',date_trunc('month',now()) + interval '1 month'
			ON CONFLICT(admin_id) DO NOTHING`,
			domain.MustID("usr"), item.id, item.name, item.email, uuid, hash, auth.Prefix(plain, 12), plain, desiredStatus)
		if err != nil {
			return mapError(err)
		}
		changed = changed || command.RowsAffected() > 0
	}
	if changed {
		return s.NotifyAllPublishedNodes(ctx)
	}
	return nil
}

func (s *Store) CreateAdmin(ctx context.Context, email, name, passwordHash string) (domain.Admin, error) {
	admin := domain.Admin{ID: domain.MustID("adm")}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO admins(id, email, name, password_hash)
		VALUES($1, lower($2), $3, $4)
		RETURNING email, name, status, last_login_at, created_at`,
		admin.ID, strings.TrimSpace(email), strings.TrimSpace(name), passwordHash,
	).Scan(&admin.Email, &admin.Name, &admin.Status, &admin.LastLogin, &admin.CreatedAt)
	return admin, mapError(err)
}

func (s *Store) FindAdminByEmail(ctx context.Context, email string) (domain.AdminAuth, error) {
	var admin domain.AdminAuth
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, name, status, last_login_at, created_at, password_hash
		FROM admins WHERE lower(email)=lower($1)`, strings.TrimSpace(email),
	).Scan(&admin.ID, &admin.Email, &admin.Name, &admin.Status, &admin.LastLogin, &admin.CreatedAt, &admin.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminAuth{}, ErrNotFound
	}
	return admin, err
}

func (s *Store) FindAdminByID(ctx context.Context, adminID string) (domain.AdminAuth, error) {
	var admin domain.AdminAuth
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, name, status, last_login_at, created_at, password_hash
		FROM admins WHERE id=$1`, adminID,
	).Scan(&admin.ID, &admin.Email, &admin.Name, &admin.Status, &admin.LastLogin, &admin.CreatedAt, &admin.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminAuth{}, ErrNotFound
	}
	return admin, err
}

func (s *Store) UpdateAdminProfile(ctx context.Context, adminID, email, name string) (domain.Admin, error) {
	var admin domain.Admin
	err := s.pool.QueryRow(ctx, `
		UPDATE admins SET email=lower($2), name=$3, updated_at=now()
		WHERE id=$1
		RETURNING id,email,name,status,last_login_at,created_at`,
		adminID, strings.TrimSpace(email), strings.TrimSpace(name),
	).Scan(&admin.ID, &admin.Email, &admin.Name, &admin.Status, &admin.LastLogin, &admin.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Admin{}, ErrNotFound
	}
	return admin, mapError(err)
}

func (s *Store) UpdateAdminPassword(ctx context.Context, adminID, currentSessionID, passwordHash string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, "UPDATE admins SET password_hash=$2, updated_at=now() WHERE id=$1", adminID, passwordHash)
	if err != nil {
		return 0, mapError(err)
	}
	if command.RowsAffected() == 0 {
		return 0, ErrNotFound
	}
	command, err = tx.Exec(ctx, `
		UPDATE admin_sessions SET revoked_at=now()
		WHERE admin_id=$1 AND id<>$2 AND revoked_at IS NULL`, adminID, currentSessionID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

func (s *Store) TouchAdminLogin(ctx context.Context, adminID string) error {
	_, err := s.pool.Exec(ctx, "UPDATE admins SET last_login_at=now(), updated_at=now() WHERE id=$1", adminID)
	return err
}

func (s *Store) CreateAdminSession(ctx context.Context, adminID string, tokenHash []byte, csrfToken, ipAddress, userAgent string, expiresAt time.Time) (string, error) {
	id := domain.MustID("ses")
	var ip any
	if parsed := net.ParseIP(ipAddress); parsed != nil {
		ip = parsed.String()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO admin_sessions(id, admin_id, token_hash, csrf_token, ip_address, user_agent, expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)`,
		id, adminID, tokenHash, csrfToken, ip, userAgent, expiresAt,
	)
	return id, mapError(err)
}

func (s *Store) FindAdminSession(ctx context.Context, tokenHash []byte) (domain.AdminSession, error) {
	var session domain.AdminSession
	err := s.pool.QueryRow(ctx, `
		SELECT s.id, s.csrf_token, s.expires_at,
		       a.id, a.email, a.name, a.status, a.last_login_at, a.created_at
		FROM admin_sessions s
		JOIN admins a ON a.id=s.admin_id
		WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at > now() AND a.status='active'`, tokenHash,
	).Scan(
		&session.ID, &session.CSRFToken, &session.ExpiresAt,
		&session.Admin.ID, &session.Admin.Email, &session.Admin.Name, &session.Admin.Status,
		&session.Admin.LastLogin, &session.Admin.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminSession{}, ErrNotFound
	}
	return session, err
}

func (s *Store) RevokeAdminSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, "UPDATE admin_sessions SET revoked_at=now() WHERE token_hash=$1", tokenHash)
	return err
}

func (s *Store) WriteAudit(ctx context.Context, adminID, action, resourceType, resourceID string, changes any, ipAddress, requestID string) error {
	data, err := marshalAuditChanges(resourceType, changes)
	if err != nil {
		return err
	}
	var ip any
	if parsed := net.ParseIP(ipAddress); parsed != nil {
		ip = parsed.String()
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_events(id, admin_id, action, resource_type, resource_id, changes, ip_address, request_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		domain.MustID("aud"), nullableText(adminID), action, resourceType, resourceID, data, ip, requestID,
	)
	return err
}

const auditRedactedValue = "[REDACTED]"

func marshalAuditChanges(resourceType string, changes any) ([]byte, error) {
	data, err := json.Marshal(changes)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	if object, ok := value.(map[string]any); ok {
		if resourceType == "node" {
			redactAuditField(object, "config")
		}
		if resourceType == "outbound" {
			redactAuditField(object, "settings")
		}
	}
	return json.Marshal(redactAuditValue(value))
}

func redactAuditField(value map[string]any, field string) {
	for key := range value {
		if strings.EqualFold(strings.TrimSpace(key), field) {
			value[key] = auditRedactedValue
		}
	}
}

func redactAuditValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if sensitiveAuditKey(key) {
				typed[key] = auditRedactedValue
				continue
			}
			typed[key] = redactAuditValue(nested)
		}
		return typed
	case []any:
		for index, nested := range typed {
			typed[index] = redactAuditValue(nested)
		}
		return typed
	default:
		return value
	}
}

func sensitiveAuditKey(key string) bool {
	normalized := strings.Map(func(character rune) rune {
		if character >= 'A' && character <= 'Z' {
			return character + ('a' - 'A')
		}
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			return character
		}
		return -1
	}, key)
	switch normalized {
	case "password", "passwordhash", "portalpassword", "portalpasswordhash",
		"privatekey", "secretkey", "serverkey", "keycontent",
		"token", "tokenhash", "bottoken", "dnsapitoken", "apitoken", "apikey",
		"communicationkey", "subscriptiontoken", "subscriptiontokenplain",
		"authorization", "credential", "credentials", "obfspassword", "uuid", "udid":
		return true
	default:
		return strings.HasSuffix(normalized, "password") ||
			strings.HasSuffix(normalized, "privatekey") ||
			strings.HasSuffix(normalized, "secretkey")
	}
}

func (s *Store) ListAuditEvents(ctx context.Context, limit int) ([]domain.AuditEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT e.id,e.admin_id,a.name,e.action,e.resource_type,e.resource_id,e.changes,
		       host(e.ip_address),e.request_id,e.created_at
		FROM audit_events e LEFT JOIN admins a ON a.id=e.admin_id
		ORDER BY e.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var item domain.AuditEvent
		if err := rows.Scan(&item.ID, &item.AdminID, &item.AdminName, &item.Action, &item.ResourceType,
			&item.ResourceID, &item.Changes, &item.IPAddress, &item.RequestID, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListAuditEventsPage(ctx context.Context, page, pageSize int, query, sortField, order string) ([]domain.AuditEvent, int, error) {
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
	sortSQL := "e.created_at"
	switch sortField {
	case "action":
		sortSQL = "e.action"
	case "resource_type":
		sortSQL = "e.resource_type"
	}
	pattern := "%" + strings.TrimSpace(query) + "%"
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events e LEFT JOIN admins a ON a.id=e.admin_id
		WHERE ($1='' OR e.action ILIKE $2 OR e.resource_type ILIKE $2 OR e.resource_id ILIKE $2 OR COALESCE(a.name,'') ILIKE $2 OR e.request_id ILIKE $2)`, query, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}
	// sortSQL/orderSQL are selected exclusively from constants above.
	rows, err := s.pool.Query(ctx, `SELECT e.id,e.admin_id,a.name,e.action,e.resource_type,e.resource_id,e.changes,
		host(e.ip_address),e.request_id,e.created_at FROM audit_events e LEFT JOIN admins a ON a.id=e.admin_id
		WHERE ($1='' OR e.action ILIKE $2 OR e.resource_type ILIKE $2 OR e.resource_id ILIKE $2 OR COALESCE(a.name,'') ILIKE $2 OR e.request_id ILIKE $2)
		ORDER BY `+sortSQL+` `+orderSQL+`, e.id DESC LIMIT $3 OFFSET $4`, query, pattern, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var item domain.AuditEvent
		if err := rows.Scan(&item.ID, &item.AdminID, &item.AdminName, &item.Action, &item.ResourceType, &item.ResourceID, &item.Changes, &item.IPAddress, &item.RequestID, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func nullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

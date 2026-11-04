package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var versionedResources = map[string]bool{
	"machines": true, "nodes": true, "users": true, "plans": true,
	"access_groups": true, "route_policies": true, "outbounds": true,
}

func (s *Store) ConfigureAdminMutation(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `SELECT set_config('cpanel.admin_mutation','on',true)`)
	return err
}

// AcquireAdminConsistencyLocks must precede the optimistic target-row lock.
// Store reference checks already take this advisory lock, so reversing the
// order here would deadlock against a direct Store mutation. User/plan handlers
// reconcile all users; lock them in telemetry's order before locking one target.
func (s *Store) AcquireAdminConsistencyLocks(ctx context.Context, resource string) error {
	if _, err := s.pool.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('cpanel:resource-consistency', 0))`); err != nil {
		return err
	}
	if resource == "users" || resource == "plans" {
		if err := lockAllTrafficUsers(ctx, s.pool); err != nil {
			return err
		}
	}
	if resource == "nodes" || resource == "route_policies" || resource == "outbounds" {
		rows, err := s.pool.Query(ctx, `SELECT id FROM nodes ORDER BY agent_id FOR UPDATE`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
		}
		rows.Close()
		return rows.Err()
	}
	return nil
}

func (s *Store) ResourceConfigVersions(ctx context.Context, resource string, ids []string) (map[string]int64, error) {
	if !versionedResources[resource] {
		return nil, fmt.Errorf("unsupported versioned resource")
	}
	rows, err := s.pool.Query(ctx, `SELECT id,config_version FROM `+resource+` WHERE id=ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := make(map[string]int64, len(ids))
	for rows.Next() {
		var id string
		var version int64
		if err := rows.Scan(&id, &version); err != nil {
			return nil, err
		}
		versions[id] = version
	}
	return versions, rows.Err()
}

func (s *Store) CheckResourceConfigVersion(ctx context.Context, resource, id string, expected int64) error {
	if !versionedResources[resource] {
		return fmt.Errorf("unsupported versioned resource")
	}
	var actual int64
	err := s.pool.QueryRow(ctx, `SELECT config_version FROM `+resource+` WHERE id=$1 AND status <> 'archived' FOR UPDATE`, id).Scan(&actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if actual != expected {
		return ErrEditVersionConflict
	}
	return nil
}

func (s *Store) SettingSectionConfigVersion(ctx context.Context, section string, lock bool) (int64, error) {
	query := `SELECT config_version FROM setting_section_versions WHERE section=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	var version int64
	err := s.pool.QueryRow(ctx, query, section).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return version, err
}

type IdempotencyResult struct {
	Status   int
	Response json.RawMessage
}

// ClaimIdempotency must be used inside BeginRequest. The insert conflict waits
// for a concurrent identical request to commit/roll back, so success, audit and
// cached encrypted response become visible atomically.
func (s *Store) ClaimIdempotency(ctx context.Context, adminID, key string, hash []byte) (*IdempotencyResult, error) {
	if s.pool.request(ctx) == nil {
		return nil, fmt.Errorf("idempotency claim requires request transaction")
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO admin_idempotency_results(admin_id,intent_key,request_hash)
		VALUES($1,$2,$3) ON CONFLICT(admin_id,intent_key) DO NOTHING`, adminID, key, hash)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 1 {
		return nil, nil
	}
	var storedHash []byte
	var result IdempotencyResult
	var status *int
	var expires time.Time
	err = s.pool.QueryRow(ctx, `SELECT request_hash,response_status,sealed_response,expires_at
		FROM admin_idempotency_results WHERE admin_id=$1 AND intent_key=$2`, adminID, key).
		Scan(&storedHash, &status, &result.Response, &expires)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(storedHash, hash) {
		return nil, ErrIdempotencyConflict
	}
	if !expires.After(time.Now()) {
		return nil, ErrIdempotencyExpired
	}
	if status == nil || len(result.Response) == 0 {
		return nil, fmt.Errorf("incomplete idempotency result")
	}
	result.Status = *status
	return &result, nil
}

func (s *Store) CompleteIdempotency(ctx context.Context, adminID, key string, hash []byte, status int, sealed json.RawMessage) error {
	tag, err := s.pool.Exec(ctx, `UPDATE admin_idempotency_results SET response_status=$4,sealed_response=$5
		WHERE admin_id=$1 AND intent_key=$2 AND request_hash=$3 AND response_status IS NULL`, adminID, key, hash, status, sealed)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrIdempotencyConflict
	}
	return err
}

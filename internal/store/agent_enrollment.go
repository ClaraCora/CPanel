package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// IssueAgentEnrollment replaces any unused credential for this machine. The
// encrypted secret is needed for the protocol's HMAC proof and is deleted in
// the same transaction that binds the Agent identity.
func (s *Store) IssueAgentEnrollment(ctx context.Context, machineID string, tokenHash []byte, sealedKey json.RawMessage, expiresAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	if err := tx.QueryRow(ctx, `SELECT id FROM machines WHERE id=$1 AND status NOT IN ('disabled','archived') FOR UPDATE`, machineID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	var enrolled bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_identities WHERE machine_id=$1)`, machineID).Scan(&enrolled); err != nil {
		return err
	}
	if enrolled {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO agent_enrollment_credentials(machine_id,token_hash,sealed_key,expires_at)
		VALUES($1,$2,$3,$4) ON CONFLICT(machine_id) DO UPDATE SET token_hash=EXCLUDED.token_hash,
		sealed_key=EXCLUDED.sealed_key,expires_at=EXCLUDED.expires_at,created_at=now()`, machineID, tokenHash, sealedKey, expiresAt); err != nil {
		return err
	}
	// Selecting V2 enrollment permanently disables legacy authentication, including
	// the interval before the first successful encrypted connection.
	if _, err := tx.Exec(ctx, `UPDATE machines SET agent_protocol='v2',updated_at=now() WHERE id=$1`, machineID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) AgentEnrollmentCredential(ctx context.Context, machineID string) (json.RawMessage, []byte, error) {
	var sealed json.RawMessage
	var hash []byte
	err := s.pool.QueryRow(ctx, `SELECT sealed_key,token_hash FROM agent_enrollment_credentials WHERE machine_id=$1 AND expires_at>now()`, machineID).Scan(&sealed, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	return sealed, hash, err
}

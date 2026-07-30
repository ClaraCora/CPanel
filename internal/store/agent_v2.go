package store

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"time"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

type AgentPanelIdentity struct {
	PrivateKey json.RawMessage
	PublicKey  []byte
}

type AgentIdentity struct {
	MachineID  string
	PublicKey  []byte
	Status     string
	EnrolledAt time.Time
	LastSeenAt *time.Time
}

func (s *Store) AgentPanelIdentity(ctx context.Context) (AgentPanelIdentity, error) {
	var identity AgentPanelIdentity
	err := s.pool.QueryRow(ctx, `SELECT private_key,public_key FROM agent_panel_identity WHERE singleton=1`).Scan(
		&identity.PrivateKey, &identity.PublicKey,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentPanelIdentity{}, ErrNotFound
	}
	return identity, err
}

func (s *Store) CreateAgentPanelIdentity(ctx context.Context, privateKey json.RawMessage, publicKey ed25519.PublicKey) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO agent_panel_identity(singleton,private_key,public_key)
		VALUES(1,$1,$2) ON CONFLICT(singleton) DO NOTHING`, privateKey, []byte(publicKey))
	return err
}

func (s *Store) AgentIdentity(ctx context.Context, machineID string) (AgentIdentity, error) {
	var identity AgentIdentity
	err := s.pool.QueryRow(ctx, `SELECT machine_id,public_key,status,enrolled_at,last_seen_at
		FROM agent_identities WHERE machine_id=$1`, machineID).Scan(
		&identity.MachineID, &identity.PublicKey, &identity.Status, &identity.EnrolledAt, &identity.LastSeenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentIdentity{}, ErrNotFound
	}
	return identity, err
}

func (s *Store) EnrollAgentIdentity(ctx context.Context, machineID string, publicKey ed25519.PublicKey) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var machineExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM machines WHERE id=$1 AND status NOT IN ('disabled','archived'))`, machineID).Scan(&machineExists); err != nil {
		return false, err
	}
	if !machineExists {
		return false, ErrNotFound
	}

	var existing []byte
	err = tx.QueryRow(ctx, `SELECT public_key FROM agent_identities WHERE machine_id=$1 FOR UPDATE`, machineID).Scan(&existing)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if _, err := tx.Exec(ctx, `INSERT INTO agent_identities(machine_id,public_key) VALUES($1,$2)`, machineID, []byte(publicKey)); err != nil {
			return false, mapError(err)
		}
	case err != nil:
		return false, err
	case !equalBytes(existing, publicKey):
		return false, ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return errors.Is(err, pgx.ErrNoRows), nil
}

func (s *Store) AgentMachineByID(ctx context.Context, machineID string) (domain.AgentMachine, error) {
	var machine domain.AgentMachine
	err := s.pool.QueryRow(ctx, `SELECT id,name,status,kernel_type,'v2',capabilities FROM machines
		WHERE id=$1 AND status NOT IN ('disabled','archived')`, machineID).Scan(
		&machine.ID, &machine.Name, &machine.Status, &machine.KernelType, &machine.TokenID, &machine.Capabilities,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AgentMachine{}, ErrNotFound
	}
	return machine, err
}

func (s *Store) TouchAgentV2(ctx context.Context, machineID string) error {
	_, err := s.pool.Exec(ctx, `UPDATE agent_identities SET last_seen_at=now() WHERE machine_id=$1 AND status='active'`, machineID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE machines SET agent_protocol='v2',agent_v2_last_seen_at=now(),updated_at=now()
		WHERE id=$1 AND status NOT IN ('disabled','archived')`, machineID)
	return err
}

func (s *Store) ResetAgentIdentity(ctx context.Context, machineID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM agent_identities WHERE machine_id=$1`, machineID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM machines WHERE id=$1 AND status <> 'archived')`, machineID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	_, err = s.pool.Exec(ctx, `UPDATE machines SET agent_protocol='legacy',agent_v2_last_seen_at=NULL,updated_at=now() WHERE id=$1`, machineID)
	return err
}

func (s *Store) LegacyAgentAllowed(ctx context.Context) bool {
	value, err := s.RawSetting(ctx, "agent", "allow_legacy_protocol")
	if err != nil {
		return true
	}
	var allowed bool
	if json.Unmarshal(value, &allowed) != nil {
		return true
	}
	return allowed
}

func (s *Store) LegacyAgentBlockers(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM machines
		WHERE status NOT IN ('disabled','archived') AND agent_protocol <> 'v2' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blockers := make([]string, 0)
	for rows.Next() {
		var machineID string
		if err := rows.Scan(&machineID); err != nil {
			return nil, err
		}
		blockers = append(blockers, machineID)
	}
	return blockers, rows.Err()
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}

package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"cpanel/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ListSettings(ctx context.Context, section string) ([]domain.Setting, error) {
	rows, err := s.pool.Query(ctx, `SELECT key,value,sensitive,version,updated_at FROM settings WHERE section=$1 ORDER BY key`, section)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Setting, 0)
	for rows.Next() {
		var item domain.Setting
		if err := rows.Scan(&item.Key, &item.Value, &item.Sensitive, &item.Version, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if item.Sensitive {
			item.Value = json.RawMessage(`{"configured":true}`)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RawSetting(ctx context.Context, section, key string) (json.RawMessage, error) {
	var value json.RawMessage
	err := s.pool.QueryRow(ctx, `SELECT value FROM settings WHERE section=$1 AND key=$2`, section, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return value, err
}

func (s *Store) SettingString(ctx context.Context, section, key, fallback string) string {
	value, err := s.RawSetting(ctx, section, key)
	if err != nil {
		return fallback
	}
	var decoded string
	if json.Unmarshal(value, &decoded) != nil || decoded == "" {
		return fallback
	}
	return decoded
}

func (s *Store) SettingInt(ctx context.Context, section, key string, fallback, minimum int) int {
	value, err := s.RawSetting(ctx, section, key)
	if err != nil {
		return fallback
	}
	var decoded int
	if json.Unmarshal(value, &decoded) != nil {
		var text string
		if json.Unmarshal(value, &text) != nil {
			return fallback
		}
		decoded, err = strconv.Atoi(text)
		if err != nil {
			return fallback
		}
	}
	if decoded < minimum {
		return fallback
	}
	return decoded
}

func (s *Store) SettingBool(ctx context.Context, section, key string, fallback bool) bool {
	value, err := s.RawSetting(ctx, section, key)
	if err != nil {
		return fallback
	}
	var decoded bool
	if json.Unmarshal(value, &decoded) == nil {
		return decoded
	}
	var text string
	if json.Unmarshal(value, &text) != nil {
		return fallback
	}
	decoded, err = strconv.ParseBool(text)
	if err != nil {
		return fallback
	}
	return decoded
}

func (s *Store) SetAgentSharedCredential(ctx context.Context, adminID string, tokenHash []byte, prefix string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO agent_shared_credentials(singleton,token_hash,token_prefix,updated_by)
		VALUES(1,$1,$2,$3) ON CONFLICT(singleton) DO UPDATE SET
		token_hash=EXCLUDED.token_hash,token_prefix=EXCLUDED.token_prefix,updated_by=EXCLUDED.updated_by,updated_at=now()`,
		tokenHash, prefix, adminID)
	return err
}

func (s *Store) UpsertSettings(ctx context.Context, section, adminID string, values map[string]json.RawMessage, sensitive map[string]bool) error {
	return s.upsertSettings(ctx, section, adminID, values, sensitive, nil, "")
}

// UpsertSettingsWithAgentCredential updates the encrypted Agent communication
// key and its authentication digest atomically. This prevents installation
// commands from receiving a key that the control endpoint does not yet accept.
func (s *Store) UpsertSettingsWithAgentCredential(
	ctx context.Context,
	section, adminID string,
	values map[string]json.RawMessage,
	sensitive map[string]bool,
	tokenHash []byte,
	prefix string,
) error {
	return s.upsertSettings(ctx, section, adminID, values, sensitive, tokenHash, prefix)
}

func (s *Store) upsertSettings(
	ctx context.Context,
	section, adminID string,
	values map[string]json.RawMessage,
	sensitive map[string]bool,
	agentTokenHash []byte,
	agentTokenPrefix string,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for key, value := range values {
		_, err := tx.Exec(ctx, `
			INSERT INTO settings(section,key,value,sensitive,updated_by)
			VALUES($1,$2,$3,$4,$5)
			ON CONFLICT(section,key) DO UPDATE SET
			value=excluded.value,sensitive=excluded.sensitive,version=settings.version+1,
			updated_by=excluded.updated_by,updated_at=now()`,
			section, key, value, sensitive[key], adminID)
		if err != nil {
			return err
		}
	}
	if len(agentTokenHash) > 0 {
		_, err := tx.Exec(ctx, `INSERT INTO agent_shared_credentials(singleton,token_hash,token_prefix,updated_by)
			VALUES(1,$1,$2,$3) ON CONFLICT(singleton) DO UPDATE SET
			token_hash=EXCLUDED.token_hash,token_prefix=EXCLUDED.token_prefix,
			updated_by=EXCLUDED.updated_by,updated_at=now()`,
			agentTokenHash, agentTokenPrefix, adminID)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

ALTER TABLE users
    ADD COLUMN portal_login TEXT,
    ADD COLUMN portal_password_hash TEXT,
    ADD COLUMN portal_enabled_at TIMESTAMPTZ;

-- Existing email addresses are stable, already unique account identifiers. Passwords
-- intentionally remain unset so an administrator must explicitly enable portal access.
UPDATE users
SET portal_login = lower(email)
WHERE role IN ('user', 'friend')
  AND email IS NOT NULL
  AND btrim(email) <> '';

CREATE UNIQUE INDEX users_portal_login_unique
    ON users(lower(portal_login))
    WHERE portal_login IS NOT NULL;

CREATE TABLE user_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    csrf_token TEXT NOT NULL,
    delegated_by_admin_id TEXT REFERENCES admins(id) ON DELETE SET NULL,
    read_only BOOLEAN NOT NULL DEFAULT false,
    ip_address INET,
    user_agent TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX user_sessions_user_idx ON user_sessions(user_id, expires_at DESC);

CREATE TABLE user_portal_grants (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    admin_id TEXT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    redeemed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX user_portal_grants_expiry_idx ON user_portal_grants(expires_at);

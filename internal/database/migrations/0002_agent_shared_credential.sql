CREATE TABLE agent_shared_credentials (
    singleton SMALLINT PRIMARY KEY DEFAULT 1 CHECK (singleton = 1),
    token_hash BYTEA NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    updated_by TEXT REFERENCES admins(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

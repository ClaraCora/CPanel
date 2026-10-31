CREATE TABLE agent_enrollment_credentials (
    machine_id TEXT PRIMARY KEY REFERENCES machines(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    sealed_key JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

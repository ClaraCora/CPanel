CREATE TABLE agent_panel_identity (
    singleton SMALLINT PRIMARY KEY DEFAULT 1 CHECK (singleton = 1),
    private_key JSONB NOT NULL,
    public_key BYTEA NOT NULL CHECK (octet_length(public_key) = 32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agent_identities (
    machine_id TEXT PRIMARY KEY REFERENCES machines(id) ON DELETE CASCADE,
    public_key BYTEA NOT NULL CHECK (octet_length(public_key) = 32),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

ALTER TABLE machines
    ADD COLUMN agent_protocol TEXT NOT NULL DEFAULT 'legacy'
        CHECK (agent_protocol IN ('legacy', 'v2')),
    ADD COLUMN agent_v2_last_seen_at TIMESTAMPTZ;

INSERT INTO settings(section,key,value,sensitive)
VALUES ('agent','allow_legacy_protocol','true'::jsonb,false)
ON CONFLICT(section,key) DO NOTHING;

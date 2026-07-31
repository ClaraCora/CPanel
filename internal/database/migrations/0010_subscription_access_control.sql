CREATE TABLE subscription_access_events (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    user_name TEXT NOT NULL DEFAULT '',
    ip_address INET,
    user_agent TEXT NOT NULL DEFAULT '',
    outcome TEXT NOT NULL CHECK (outcome IN ('allowed', 'blocked', 'not_found', 'failed')),
    status_code INTEGER NOT NULL CHECK (status_code BETWEEN 100 AND 599),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX subscription_access_events_created_idx ON subscription_access_events(created_at DESC);

INSERT INTO settings(section,key,value,sensitive)
VALUES
    ('subscription','block_browser_access','false'::jsonb,false),
    ('subscription','ua_whitelist','""'::jsonb,false),
    ('retention','subscription_access_days','30'::jsonb,false)
ON CONFLICT(section,key) DO NOTHING;

INSERT INTO settings(section,key,value,sensitive)
VALUES ('security','password_min_length','8'::jsonb,false)
ON CONFLICT(section,key) DO UPDATE SET
    value='8'::jsonb,
    sensitive=false,
    version=settings.version+1,
    updated_at=now();

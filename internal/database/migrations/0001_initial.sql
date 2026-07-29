CREATE TABLE admins (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX admins_email_unique ON admins (lower(email));

CREATE TABLE admin_sessions (
    id TEXT PRIMARY KEY,
    admin_id TEXT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    csrf_token TEXT NOT NULL,
    ip_address INET,
    user_agent TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX admin_sessions_admin_idx ON admin_sessions (admin_id, expires_at DESC);

CREATE TABLE machines (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    region TEXT NOT NULL DEFAULT '',
    host TEXT NOT NULL DEFAULT '',
    labels JSONB NOT NULL DEFAULT '{}'::jsonb,
    notes TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'online', 'offline', 'disabled', 'archived')),
    agent_version TEXT NOT NULL DEFAULT '',
    kernel_type TEXT NOT NULL DEFAULT 'singbox' CHECK (kernel_type IN ('singbox', 'xray')),
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_heartbeat_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE machine_credentials (
    id TEXT PRIMARY KEY,
    machine_id TEXT NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX machine_credentials_machine_idx ON machine_credentials (machine_id, created_at DESC);

CREATE TABLE access_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'archived')),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE route_policies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'disabled', 'archived')),
    current_revision INTEGER NOT NULL DEFAULT 0,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outbounds (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    tag TEXT NOT NULL UNIQUE,
    protocol TEXT NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    proxy_tag TEXT NOT NULL DEFAULT '',
    kernel_support TEXT[] NOT NULL DEFAULT ARRAY['singbox','xray']::TEXT[],
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE route_policy_revisions (
    id TEXT PRIMARY KEY,
    route_policy_id TEXT NOT NULL REFERENCES route_policies(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    rules JSONB NOT NULL DEFAULT '[]'::jsonb,
    compiled_singbox JSONB,
    compiled_xray JSONB,
    validation_errors JSONB NOT NULL DEFAULT '[]'::jsonb,
    published_by TEXT REFERENCES admins(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(route_policy_id, revision)
);

CREATE TABLE nodes (
    id TEXT PRIMARY KEY,
    agent_id BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    machine_id TEXT NOT NULL REFERENCES machines(id),
    route_policy_id TEXT REFERENCES route_policies(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    protocol TEXT NOT NULL,
    listen_ip TEXT NOT NULL DEFAULT '0.0.0.0',
    server_port INTEGER NOT NULL CHECK (server_port BETWEEN 1 AND 65535),
    kernel_type TEXT NOT NULL DEFAULT 'singbox' CHECK (kernel_type IN ('singbox', 'xray')),
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'disabled', 'error', 'archived')),
    current_revision INTEGER NOT NULL DEFAULT 0,
    applied_revision INTEGER NOT NULL DEFAULT 0,
    last_report_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(machine_id, server_port)
);
CREATE INDEX nodes_machine_idx ON nodes (machine_id, status);

CREATE TABLE node_revisions (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    config JSONB NOT NULL,
    validation_errors JSONB NOT NULL DEFAULT '[]'::jsonb,
    published_by TEXT REFERENCES admins(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(node_id, revision)
);

CREATE TABLE access_group_nodes (
    access_group_id TEXT NOT NULL REFERENCES access_groups(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(access_group_id, node_id)
);

CREATE TABLE plans (
    id TEXT PRIMARY KEY,
    access_group_id TEXT NOT NULL REFERENCES access_groups(id),
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'archived')),
    traffic_limit_bytes BIGINT NOT NULL DEFAULT 0 CHECK (traffic_limit_bytes >= 0),
    speed_limit_mbps INTEGER NOT NULL DEFAULT 0 CHECK (speed_limit_mbps >= 0),
    device_limit INTEGER NOT NULL DEFAULT 0 CHECK (device_limit >= 0),
    reset_strategy TEXT NOT NULL DEFAULT 'calendar_month' CHECK (reset_strategy = 'calendar_month'),
    default_valid_days INTEGER NOT NULL DEFAULT 0 CHECK (default_valid_days >= 0),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    agent_id BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'friend')),
    plan_id TEXT REFERENCES plans(id) ON DELETE SET NULL,
    access_group_override_id TEXT REFERENCES access_groups(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    email TEXT,
    uuid TEXT NOT NULL UNIQUE,
    subscription_token_hash BYTEA NOT NULL UNIQUE,
    subscription_token_prefix TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'expired', 'archived')),
    traffic_limit_override_bytes BIGINT CHECK (traffic_limit_override_bytes >= 0),
    speed_limit_override_mbps INTEGER CHECK (speed_limit_override_mbps >= 0),
    device_limit_override INTEGER CHECK (device_limit_override >= 0),
    traffic_used_bytes BIGINT NOT NULL DEFAULT 0 CHECK (traffic_used_bytes >= 0),
    traffic_reset_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_unique ON users (lower(email)) WHERE email IS NOT NULL;
CREATE INDEX users_plan_idx ON users (plan_id, status);

CREATE TABLE user_devices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    ip_address INET NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    online BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(user_id, node_id, ip_address)
);

CREATE TABLE traffic_daily (
    day DATE NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    upload_bytes BIGINT NOT NULL DEFAULT 0 CHECK (upload_bytes >= 0),
    download_bytes BIGINT NOT NULL DEFAULT 0 CHECK (download_bytes >= 0),
    PRIMARY KEY(day, user_id, node_id)
);

CREATE TABLE machine_metrics (
    machine_id TEXT NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    sampled_at TIMESTAMPTZ NOT NULL,
    metrics JSONB NOT NULL,
    PRIMARY KEY(machine_id, sampled_at)
);

CREATE TABLE node_metrics (
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    sampled_at TIMESTAMPTZ NOT NULL,
    metrics JSONB NOT NULL,
    PRIMARY KEY(node_id, sampled_at)
);

CREATE TABLE settings (
    section TEXT NOT NULL,
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    sensitive BOOLEAN NOT NULL DEFAULT false,
    version INTEGER NOT NULL DEFAULT 1,
    updated_by TEXT REFERENCES admins(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(section, key)
);

CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    admin_id TEXT REFERENCES admins(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL DEFAULT '',
    changes JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address INET,
    request_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_created_idx ON audit_events (created_at DESC);

CREATE TABLE control_changes (
    cursor BIGSERIAL PRIMARY KEY,
    machine_id TEXT NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    node_id TEXT REFERENCES nodes(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX control_changes_machine_cursor_idx ON control_changes (machine_id, cursor);

CREATE TABLE telemetry_batches (
    machine_id TEXT NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(machine_id, idempotency_key)
);

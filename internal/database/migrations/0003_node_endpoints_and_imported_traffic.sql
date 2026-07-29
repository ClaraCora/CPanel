CREATE TABLE node_endpoints (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(node_id, name, host, port)
);
CREATE INDEX node_endpoints_node_idx ON node_endpoints (node_id, status, sort_order);

CREATE TABLE imported_node_traffic_daily (
    day DATE NOT NULL,
    node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    upload_bytes BIGINT NOT NULL DEFAULT 0 CHECK (upload_bytes >= 0),
    download_bytes BIGINT NOT NULL DEFAULT 0 CHECK (download_bytes >= 0),
    PRIMARY KEY(day, node_id)
);

CREATE TABLE imported_user_traffic_daily (
    day DATE NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    upload_bytes BIGINT NOT NULL DEFAULT 0 CHECK (upload_bytes >= 0),
    download_bytes BIGINT NOT NULL DEFAULT 0 CHECK (download_bytes >= 0),
    PRIMARY KEY(day, user_id)
);

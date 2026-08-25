ALTER TABLE node_endpoints
    ADD COLUMN IF NOT EXISTS access_scope TEXT NOT NULL DEFAULT 'default'
        CHECK (access_scope IN ('default', 'admin'));

CREATE INDEX IF NOT EXISTS node_endpoints_access_scope_idx
    ON node_endpoints (node_id, access_scope, status, sort_order);

ALTER TABLE route_policies
    ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'default'
        CHECK (scope IN ('default', 'admin', 'member'));

ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS admin_route_policy_id TEXT REFERENCES route_policies(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS member_route_policy_id TEXT REFERENCES route_policies(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS nodes_route_policy_scope_idx
    ON nodes (route_policy_id, admin_route_policy_id, member_route_policy_id);

UPDATE route_policies SET scope = 'default' WHERE scope IS NULL OR scope = '';

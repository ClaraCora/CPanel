ALTER TABLE route_policies
    ADD COLUMN IF NOT EXISTS default_outbound_tag TEXT NOT NULL DEFAULT '';

ALTER TABLE user_access_ips
    ADD COLUMN location_scope TEXT NOT NULL DEFAULT '',
    ADD COLUMN location_country_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN location_country TEXT NOT NULL DEFAULT '',
    ADD COLUMN location_province TEXT NOT NULL DEFAULT '',
    ADD COLUMN location_city TEXT NOT NULL DEFAULT '',
    ADD COLUMN location_isp TEXT NOT NULL DEFAULT '',
    ADD COLUMN location_updated_at TIMESTAMPTZ,
    ADD CONSTRAINT user_access_ips_location_scope_check
        CHECK (location_scope IN ('', 'public', 'private'));

CREATE INDEX user_access_ips_address_idx
    ON user_access_ips(ip_address);

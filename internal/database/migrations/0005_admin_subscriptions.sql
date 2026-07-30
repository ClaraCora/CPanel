ALTER TABLE users
    ADD COLUMN admin_id TEXT UNIQUE REFERENCES admins(id) ON DELETE CASCADE;

ALTER TABLE users
    DROP CONSTRAINT users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'user', 'friend'));

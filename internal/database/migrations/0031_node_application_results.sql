-- Fetching configuration is not evidence that a kernel has applied it.
ALTER TABLE nodes ADD COLUMN received_revision integer NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN attempted_revision integer NOT NULL DEFAULT 0;
ALTER TABLE nodes ADD COLUMN apply_status text NOT NULL DEFAULT '';
ALTER TABLE nodes ADD COLUMN apply_stage text NOT NULL DEFAULT '';

-- Old applied_revision values came from ordinary telemetry, so cannot be
-- treated as confirmed application results after this migration.
UPDATE nodes SET received_revision=applied_revision,applied_revision=0;

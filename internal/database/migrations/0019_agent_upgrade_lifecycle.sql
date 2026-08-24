ALTER TABLE machines
    ADD COLUMN IF NOT EXISTS agent_upgrade_target_version TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS agent_upgrade_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS agent_upgrade_acknowledged_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS agent_upgrade_completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS agent_upgrade_failed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS agent_upgrade_error TEXT NOT NULL DEFAULT '';

-- Existing in-flight records were created before explicit lifecycle states
-- existed. Keep them visible as dispatched instead of silently treating them
-- as completed on the next heartbeat.
UPDATE machines
SET agent_upgrade_status = CASE
        WHEN agent_upgrade_task_id IS NULL THEN ''
        WHEN agent_upgrade_dispatched_at IS NULL THEN 'queued'
        ELSE 'dispatched'
    END
WHERE agent_upgrade_status = '';

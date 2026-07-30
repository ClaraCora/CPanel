WITH ranked AS (
    SELECT ctid,row_number() OVER (PARTITION BY machine_id ORDER BY sampled_at DESC,ctid DESC) AS position
    FROM machine_metrics
)
DELETE FROM machine_metrics current
USING ranked
WHERE current.ctid = ranked.ctid AND ranked.position > 1;

WITH ranked AS (
    SELECT ctid,row_number() OVER (PARTITION BY node_id ORDER BY sampled_at DESC,ctid DESC) AS position
    FROM node_metrics
)
DELETE FROM node_metrics current
USING ranked
WHERE current.ctid = ranked.ctid AND ranked.position > 1;

CREATE UNIQUE INDEX machine_metrics_latest_unique ON machine_metrics(machine_id);
CREATE UNIQUE INDEX node_metrics_latest_unique ON node_metrics(node_id);

ALTER TABLE machines
    ADD COLUMN agent_upgrade_task_id TEXT,
    ADD COLUMN agent_upgrade_requested_at TIMESTAMPTZ,
    ADD COLUMN agent_upgrade_dispatched_at TIMESTAMPTZ;

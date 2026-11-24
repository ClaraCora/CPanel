CREATE TABLE agent_upgrade_batches (
    id text PRIMARY KEY,
    target_version text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX agent_upgrade_batches_latest ON agent_upgrade_batches(created_at DESC, id DESC);
CREATE TABLE agent_upgrade_batch_items (
    batch_id text NOT NULL REFERENCES agent_upgrade_batches ON DELETE CASCADE,
    machine_id text NOT NULL,
    machine_name text NOT NULL,
    agent_version text NOT NULL,
    task_id text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('queued','dispatched','acknowledged','succeeded','already_current','failed','timed_out')),
    error text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(batch_id, machine_id)
);
CREATE INDEX agent_upgrade_batch_items_task ON agent_upgrade_batch_items(task_id) WHERE task_id <> '';

-- The machine's single command slot remains the transport for both V1 and V2.
-- Mirror just its state transitions so completed batches survive later commands.
CREATE FUNCTION cpanel_track_upgrade_batch() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.agent_upgrade_task_id IS NULL THEN RETURN NEW; END IF;
    IF NEW.status IN ('disabled','archived') AND NEW.agent_upgrade_task_id = OLD.agent_upgrade_task_id
       AND NEW.agent_upgrade_status IN ('queued','dispatched','acknowledged')
       AND EXISTS (SELECT 1 FROM agent_upgrade_batch_items WHERE task_id=OLD.agent_upgrade_task_id) THEN
        NEW.agent_upgrade_status := 'failed';
        NEW.agent_upgrade_failed_at := now();
        NEW.agent_upgrade_error := '服务器已停用或归档';
    END IF;
    IF NEW.agent_upgrade_task_id IS DISTINCT FROM OLD.agent_upgrade_task_id OR NEW.status IN ('disabled','archived') THEN
        UPDATE agent_upgrade_batch_items SET status='failed',error='服务器或升级任务已变更',updated_at=now()
        WHERE task_id=OLD.agent_upgrade_task_id AND status IN ('queued','dispatched','acknowledged');
    ELSIF NEW.agent_upgrade_status IS DISTINCT FROM OLD.agent_upgrade_status THEN
        UPDATE agent_upgrade_batch_items SET status=NEW.agent_upgrade_status,agent_version=NEW.agent_version,
            error=NEW.agent_upgrade_error,updated_at=now()
        WHERE task_id=NEW.agent_upgrade_task_id AND status IN ('queued','dispatched','acknowledged','timed_out');
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER cpanel_track_upgrade_batch BEFORE UPDATE OF agent_upgrade_task_id,agent_upgrade_status,status ON machines
FOR EACH ROW WHEN (OLD.agent_upgrade_task_id IS NOT NULL AND
    (OLD.agent_upgrade_task_id IS DISTINCT FROM NEW.agent_upgrade_task_id OR OLD.agent_upgrade_status IS DISTINCT FROM NEW.agent_upgrade_status OR
     (OLD.status IS DISTINCT FROM NEW.status AND NEW.status IN ('disabled','archived'))))
EXECUTE FUNCTION cpanel_track_upgrade_batch();

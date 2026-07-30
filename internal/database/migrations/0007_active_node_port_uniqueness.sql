ALTER TABLE nodes DROP CONSTRAINT IF EXISTS nodes_machine_id_server_port_key;

CREATE UNIQUE INDEX nodes_machine_port_active_unique
    ON nodes(machine_id,server_port)
    WHERE status <> 'archived';

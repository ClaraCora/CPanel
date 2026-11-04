ALTER TABLE machines ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE nodes ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE plans ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE access_groups ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE route_policies ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE outbounds ADD COLUMN config_version BIGINT NOT NULL DEFAULT 1;

CREATE FUNCTION bump_resource_config_version() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    excluded TEXT[] := ARRAY['config_version','created_at','updated_at'];
    old_config JSONB;
    new_config JSONB;
BEGIN
    IF TG_TABLE_NAME='machines' THEN
        excluded := excluded || ARRAY['status','agent_version','kernel_type','capabilities','last_heartbeat_at',
            'agent_protocol','agent_v2_last_seen_at','agent_upgrade_task_id','agent_upgrade_requested_at',
            'agent_upgrade_dispatched_at','agent_upgrade_target_version','agent_upgrade_status',
            'agent_upgrade_acknowledged_at','agent_upgrade_completed_at','agent_upgrade_failed_at','agent_upgrade_error'];
    ELSIF TG_TABLE_NAME='nodes' THEN
        excluded := excluded || ARRAY['applied_revision','received_revision','attempted_revision','apply_status','apply_stage','last_report_at','last_error','current_revision'];
    ELSIF TG_TABLE_NAME='users' THEN
        excluded := excluded || ARRAY['traffic_used_bytes','traffic_reset_at','subscription_token_hash','subscription_token_plain','subscription_token_prefix'];
    END IF;
    old_config := to_jsonb(OLD) - excluded;
    new_config := to_jsonb(NEW) - excluded;
    IF TG_TABLE_NAME='machines' THEN
        -- Presence transitions are runtime data; administrative disable/archive
        -- are configuration and must conflict with a stale editor.
        old_config := old_config || jsonb_build_object('administrative_status',CASE WHEN OLD.status IN ('disabled','archived') THEN OLD.status ELSE 'active' END);
        new_config := new_config || jsonb_build_object('administrative_status',CASE WHEN NEW.status IN ('disabled','archived') THEN NEW.status ELSE 'active' END);
        IF current_setting('cpanel.admin_mutation',true)='on' THEN
            old_config := old_config || jsonb_build_object('configured_kernel',OLD.kernel_type);
            new_config := new_config || jsonb_build_object('configured_kernel',NEW.kernel_type);
        END IF;
    END IF;
    NEW.config_version := GREATEST(NEW.config_version,OLD.config_version + CASE WHEN new_config IS DISTINCT FROM old_config THEN 1 ELSE 0 END);
    RETURN NEW;
END;
$$;

CREATE TRIGGER machines_config_version BEFORE UPDATE ON machines FOR EACH ROW EXECUTE FUNCTION bump_resource_config_version();
CREATE TRIGGER nodes_config_version BEFORE UPDATE ON nodes FOR EACH ROW EXECUTE FUNCTION bump_resource_config_version();
CREATE TRIGGER users_config_version BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION bump_resource_config_version();
CREATE TRIGGER plans_config_version BEFORE UPDATE ON plans FOR EACH ROW EXECUTE FUNCTION bump_resource_config_version();
CREATE TRIGGER access_groups_config_version BEFORE UPDATE ON access_groups FOR EACH ROW EXECUTE FUNCTION bump_resource_config_version();
CREATE TRIGGER route_policies_config_version BEFORE UPDATE ON route_policies FOR EACH ROW EXECUTE FUNCTION bump_resource_config_version();
CREATE TRIGGER outbounds_config_version BEFORE UPDATE ON outbounds FOR EACH ROW EXECUTE FUNCTION bump_resource_config_version();

CREATE FUNCTION bump_parent_config_version() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE old_id TEXT; new_id TEXT;
BEGIN
    IF TG_OP='UPDATE' AND to_jsonb(NEW)-'updated_at' IS NOT DISTINCT FROM to_jsonb(OLD)-'updated_at' THEN RETURN NULL; END IF;
    IF TG_TABLE_NAME='access_group_nodes' THEN
        IF TG_OP<>'INSERT' THEN old_id := OLD.access_group_id; END IF;
        IF TG_OP<>'DELETE' THEN new_id := NEW.access_group_id; END IF;
        UPDATE access_groups SET config_version=config_version+1 WHERE id=old_id OR id=new_id;
    ELSE
        IF TG_OP<>'INSERT' THEN old_id := OLD.node_id; END IF;
        IF TG_OP<>'DELETE' THEN new_id := NEW.node_id; END IF;
        UPDATE nodes SET config_version=config_version+1 WHERE id=old_id OR id=new_id;
    END IF;
    RETURN NULL;
END;
$$;
CREATE TRIGGER node_drafts_config_version AFTER INSERT OR UPDATE OR DELETE ON node_drafts FOR EACH ROW EXECUTE FUNCTION bump_parent_config_version();
CREATE TRIGGER node_endpoints_config_version AFTER INSERT OR UPDATE OR DELETE ON node_endpoints FOR EACH ROW EXECUTE FUNCTION bump_parent_config_version();
CREATE TRIGGER access_group_nodes_config_version AFTER INSERT OR UPDATE OR DELETE ON access_group_nodes FOR EACH ROW EXECUTE FUNCTION bump_parent_config_version();

CREATE TABLE setting_section_versions (
    section TEXT PRIMARY KEY,
    config_version BIGINT NOT NULL DEFAULT 1
);
INSERT INTO setting_section_versions(section) SELECT DISTINCT section FROM settings;
INSERT INTO setting_section_versions(section) VALUES ('site'),('agent'),('security'),('node_defaults'),('certificate'),('subscription'),('retention'),('tgbot') ON CONFLICT DO NOTHING;
CREATE FUNCTION bump_setting_section_version() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE target_section TEXT;
BEGIN
    IF TG_OP='UPDATE' AND NEW.value IS NOT DISTINCT FROM OLD.value AND NEW.sensitive IS NOT DISTINCT FROM OLD.sensitive THEN RETURN NULL; END IF;
    IF TG_OP='DELETE' THEN target_section := OLD.section; ELSE target_section := NEW.section; END IF;
    INSERT INTO setting_section_versions(section,config_version) VALUES(target_section,2)
    ON CONFLICT(section) DO UPDATE SET config_version=setting_section_versions.config_version+1;
    RETURN NULL;
END;
$$;
CREATE TRIGGER settings_section_version AFTER INSERT OR UPDATE OR DELETE ON settings FOR EACH ROW EXECUTE FUNCTION bump_setting_section_version();

CREATE TABLE admin_idempotency_results (
    admin_id TEXT NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    intent_key TEXT NOT NULL,
    request_hash BYTEA NOT NULL,
    response_status INTEGER,
    sealed_response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now()+interval '7 days',
    PRIMARY KEY(admin_id,intent_key)
);
CREATE INDEX admin_idempotency_expiry_idx ON admin_idempotency_results(expires_at);

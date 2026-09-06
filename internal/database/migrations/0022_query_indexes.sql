CREATE INDEX IF NOT EXISTS audit_events_admin_action_created_idx
    ON audit_events(admin_id, action, created_at DESC);
CREATE INDEX IF NOT EXISTS subscription_access_events_user_created_idx
    ON subscription_access_events(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS subscription_access_events_ip_created_idx
    ON subscription_access_events(ip_address, created_at DESC);
CREATE INDEX IF NOT EXISTS user_access_ips_user_seen_idx
    ON user_access_ips(user_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS user_access_ips_address_seen_idx
    ON user_access_ips(ip_address, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS user_devices_user_node_seen_idx
    ON user_devices(user_id, node_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS traffic_daily_day_node_user_idx
    ON traffic_daily(day DESC, node_id, user_id);
CREATE INDEX IF NOT EXISTS imported_node_traffic_daily_day_node_idx
    ON imported_node_traffic_daily(day DESC, node_id);
CREATE INDEX IF NOT EXISTS imported_user_traffic_daily_day_user_idx
    ON imported_user_traffic_daily(day DESC, user_id);

-- Monthly quota reconciliation must remain inexpensive for frequent reports.
CREATE INDEX IF NOT EXISTS traffic_daily_user_day_idx ON traffic_daily(user_id,day);
CREATE INDEX IF NOT EXISTS imported_user_traffic_daily_user_day_idx ON imported_user_traffic_daily(user_id,day);

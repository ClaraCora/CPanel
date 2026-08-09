CREATE TABLE telegram_bot_state (
    singleton SMALLINT PRIMARY KEY DEFAULT 1 CHECK (singleton = 1),
    bot_fingerprint TEXT NOT NULL DEFAULT '',
    update_offset BIGINT NOT NULL DEFAULT 0 CHECK (update_offset >= 0),
    last_report_day DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO telegram_bot_state(singleton) VALUES(1)
ON CONFLICT(singleton) DO NOTHING;

INSERT INTO settings(section,key,value,sensitive)
VALUES
    ('tgbot','enabled','false'::jsonb,false),
    ('tgbot','admin_telegram_id','""'::jsonb,false),
    ('tgbot','daily_report_enabled','true'::jsonb,false),
    ('tgbot','daily_report_time','"09:00"'::jsonb,false)
ON CONFLICT(section,key) DO NOTHING;

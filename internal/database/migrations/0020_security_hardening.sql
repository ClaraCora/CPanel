INSERT INTO settings(section,key,value,sensitive)
VALUES ('security','session_ttl_minutes','720'::jsonb,false)
ON CONFLICT(section,key) DO NOTHING;

-- Enforce the application-owned sensitivity schema for existing rows too.
UPDATE settings SET sensitive=true
WHERE (section='agent' AND key='communication_key')
   OR (section='certificate' AND key='dns_api_token')
   OR (section='tgbot' AND key='bot_token');

-- Older audit rows stored complete request payloads. Preserve harmless change
-- metadata while removing fields that can be used as connection credentials.
UPDATE audit_events
SET changes = jsonb_set(changes, '{config}', '"[REDACTED]"'::jsonb, false)
WHERE resource_type='node' AND changes ? 'config';

UPDATE audit_events
SET changes = jsonb_set(changes, '{settings}', '"[REDACTED]"'::jsonb, false)
WHERE resource_type='outbound' AND changes ? 'settings';

UPDATE audit_events
SET changes = jsonb_set(changes, '{uuid}', '"[REDACTED]"'::jsonb, false)
WHERE resource_type='user' AND changes ? 'uuid';

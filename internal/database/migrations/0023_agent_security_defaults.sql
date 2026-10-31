-- Preserve explicitly enabled legacy access only while active legacy machines
-- still need it. New installations and completed migrations default to V2 only.
UPDATE settings SET value='false'::jsonb
WHERE section='agent' AND key='allow_legacy_protocol'
  AND NOT EXISTS (
    SELECT 1 FROM machines m WHERE m.status NOT IN ('disabled','archived')
      AND m.agent_protocol <> 'v2'
      AND NOT EXISTS (SELECT 1 FROM agent_identities i WHERE i.machine_id=m.id)
  );

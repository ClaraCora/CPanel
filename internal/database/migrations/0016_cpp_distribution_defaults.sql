-- Preserve custom installer URLs while migrating the former built-in default.
UPDATE settings
SET value = '"https://raw.githubusercontent.com/ClaraCora/CPP/main/corade-install.sh"'::jsonb,
    version = version + 1,
    updated_at = now()
WHERE section = 'agent'
  AND key = 'installer_url'
  AND value = '"https://raw.githubusercontent.com/ClaraCora/CPanelde/main/install.sh"'::jsonb;

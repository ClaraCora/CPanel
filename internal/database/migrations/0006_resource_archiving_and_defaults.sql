ALTER TABLE route_policies DROP CONSTRAINT IF EXISTS route_policies_status_check;
UPDATE route_policies SET status = 'published' WHERE status = 'draft';
ALTER TABLE route_policies ALTER COLUMN status SET DEFAULT 'published';
ALTER TABLE route_policies ADD CONSTRAINT route_policies_status_check
    CHECK (status IN ('published', 'disabled', 'archived'));

ALTER TABLE machines ALTER COLUMN kernel_type SET DEFAULT 'xray';
ALTER TABLE nodes ALTER COLUMN kernel_type SET DEFAULT 'xray';

INSERT INTO settings(section,key,value,sensitive)
VALUES
    ('agent','installer_url','"https://raw.githubusercontent.com/ClaraCora/CPanelde/main/install.sh"'::jsonb,false),
    ('node_defaults','default_kernel','"xray"'::jsonb,false)
ON CONFLICT(section,key) DO UPDATE SET
    value=EXCLUDED.value,sensitive=false,version=settings.version+1,updated_at=now();

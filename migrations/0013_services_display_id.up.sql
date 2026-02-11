-- Human-friendly service identifier shown in UI (keeps UUID primary key).
-- Format: SRV-00001 (5 digits).

CREATE SEQUENCE IF NOT EXISTS service_display_seq START 1;

ALTER TABLE services
  ADD COLUMN IF NOT EXISTS display_id TEXT;

-- Backfill existing rows.
WITH todo AS (
  SELECT id, nextval('service_display_seq') AS n
  FROM services
  WHERE display_id IS NULL
  ORDER BY created_at ASC
)
UPDATE services s
SET display_id = ('SRV-' || lpad(todo.n::text, 5, '0'))
FROM todo
WHERE s.id = todo.id;

ALTER TABLE services
  ALTER COLUMN display_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS services_display_id_uidx ON services(display_id);

-- Default for future inserts.
ALTER TABLE services
  ALTER COLUMN display_id
  SET DEFAULT ('SRV-' || lpad(nextval('service_display_seq')::text, 5, '0'));



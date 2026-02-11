DROP INDEX IF EXISTS services_display_id_uidx;

ALTER TABLE services
  DROP COLUMN IF EXISTS display_id;

DROP SEQUENCE IF EXISTS service_display_seq;



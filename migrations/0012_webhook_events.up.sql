CREATE TABLE IF NOT EXISTS webhook_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
  topic TEXT NOT NULL,
  event_id TEXT NOT NULL,
  payload_hash TEXT,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (shop_id, topic, event_id)
);

CREATE INDEX IF NOT EXISTS webhook_events_processed_at_idx ON webhook_events(processed_at);



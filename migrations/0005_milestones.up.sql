CREATE TABLE IF NOT EXISTS milestones (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
  sequence INT NOT NULL,
  amount NUMERIC(12,2) NOT NULL,
  status TEXT NOT NULL, -- locked | unpaid | paid

  -- Draft Orders payment rail (MVP)
  draft_order_id TEXT,
  checkout_url TEXT,

  paid_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE (service_id, sequence)
);



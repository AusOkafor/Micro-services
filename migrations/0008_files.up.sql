CREATE TABLE IF NOT EXISTS files (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
  uploaded_by TEXT NOT NULL, -- merchant | client
  file_url TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'preview', -- preview | deliverable | other
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);



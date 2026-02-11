CREATE TABLE IF NOT EXISTS services (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
  shopify_order_id TEXT NOT NULL,
  shopify_product_id TEXT,
  client_email TEXT,
  client_name TEXT,
  total_amount NUMERIC(12,2) NOT NULL,
  currency TEXT NOT NULL DEFAULT 'USD',
  status TEXT NOT NULL,

  -- Snapshot at creation time; changes to service_product_configs never mutate existing services.
  service_config_snapshot JSONB NOT NULL,

  -- Only true when Completed due to a merchant override (must be backed by admin_actions + audit_logs).
  completed_via_override BOOLEAN NOT NULL DEFAULT FALSE,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE (shop_id, shopify_order_id)
);



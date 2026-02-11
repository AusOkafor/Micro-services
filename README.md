## Micro-service (Shopify Service Workflow App)

Backend for a Shopify app that supports service deposits, milestone payments, approval-gated final payments, and a token-based client portal.

### Local dev

- Start Postgres:

```bash
docker compose up -d
```

- Copy config template:

```bash
copy config\\env.example .env
```

- Run migrations and start API (once implemented):

```bash
go run ./cmd/api
```

### Supabase

Set these in your environment (recommended):
- `DATABASE_URL`: Supabase pooler/pgbouncer connection string (runtime)
- `DIRECT_URL`: direct connection string (migrations)

Run migrations (recommended against Supabase before starting API):

```bash
go run ./cmd/dev/migrate
```

### Dev: simulate webhooks locally

Create a payload JSON file (see `examples/webhooks/`), then run:

```bash
go run ./cmd/dev/simwebhook -secret YOUR_WEBHOOK_SECRET -shop your-store.myshopify.com -topic orders/paid -payload .\\payload.json
```

Example:

```bash
go run ./cmd/dev/simwebhook -secret YOUR_WEBHOOK_SECRET -shop your-store.myshopify.com -topic orders/paid -payload .\\examples\\webhooks\\orders_paid_service.json
```

### Dev: one-command end-to-end seed + webhook

This seeds:
- a `shops` row
- a default 50/50 service product config for a product id
- then posts a signed `orders/paid` webhook to create a service + milestones + portal token

```bash
go run ./cmd/dev/devflow -shop your-store.myshopify.com -product-id 1234567890 -webhook-secret YOUR_WEBHOOK_SECRET
```



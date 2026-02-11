package serviceproduct

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type Record struct {
	ID              string          `json:"id"`
	ShopID           string          `json:"shopId"`
	ShopifyProductID string          `json:"shopifyProductId"`
	Config           json.RawMessage `json:"config"`
	CreatedAt        string          `json:"createdAt"`
	UpdatedAt        string          `json:"updatedAt"`
}

func (r *Repository) Upsert(ctx context.Context, shopID, shopifyProductID string, cfg json.RawMessage) (*Record, error) {
	const q = `
INSERT INTO service_product_configs (shop_id, shopify_product_id, config)
VALUES ($1, $2, $3)
ON CONFLICT (shop_id, shopify_product_id) DO UPDATE SET
  config = EXCLUDED.config,
  updated_at = NOW()
RETURNING id, shop_id, shopify_product_id, config, created_at::text, updated_at::text
`
	rec := &Record{}
	if err := r.db.QueryRow(ctx, q, shopID, shopifyProductID, cfg).Scan(
		&rec.ID, &rec.ShopID, &rec.ShopifyProductID, &rec.Config, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return rec, nil
}

func (r *Repository) List(ctx context.Context, shopID string) ([]Record, error) {
	const q = `
SELECT id, shop_id, shopify_product_id, config, created_at::text, updated_at::text
FROM service_product_configs
WHERE shop_id = $1
ORDER BY updated_at DESC
`
	rows, err := r.db.Query(ctx, q, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.ID, &rec.ShopID, &rec.ShopifyProductID, &rec.Config, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}



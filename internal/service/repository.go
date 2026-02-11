package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	ID                   string          `json:"id"`
	DisplayID            string          `json:"displayId"`
	ShopID                string          `json:"shopId"`
	ShopifyOrderID        string          `json:"shopifyOrderId"`
	ShopifyProductID      string          `json:"shopifyProductId,omitempty"`
	ClientEmail           string          `json:"clientEmail,omitempty"`
	ClientName            string          `json:"clientName,omitempty"`
	TotalAmount           string          `json:"totalAmount"`
	Currency              string          `json:"currency"`
	Status                Status          `json:"status"`
	ServiceConfigSnapshot json.RawMessage `json:"serviceConfigSnapshot"`
	CompletedViaOverride  bool            `json:"completedViaOverride"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
}

type ListItem struct {
	ID              string          `json:"id"`
	DisplayID       string          `json:"displayId"`
	ShopID           string          `json:"shopId"`
	ShopifyOrderID   string          `json:"shopifyOrderId"`
	ShopifyProductID string          `json:"shopifyProductId,omitempty"`
	ClientEmail      string          `json:"clientEmail,omitempty"`
	ClientName       string          `json:"clientName,omitempty"`
	PaidAmount       string          `json:"paidAmount"`
	TotalAmount      string          `json:"totalAmount"`
	Currency         string          `json:"currency"`
	Status           Status          `json:"status"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	// Keep snapshot available for future display, but list UIs should not rely on its schema.
	ServiceConfigSnapshot json.RawMessage `json:"serviceConfigSnapshot"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByShop(ctx context.Context, shopID string) ([]ListItem, error) {
	const q = `
SELECT s.id, s.display_id, s.shop_id, s.shopify_order_id, s.shopify_product_id, s.client_email, s.client_name,
       COALESCE(SUM(CASE WHEN m.status = 'paid' THEN m.amount ELSE 0 END), 0)::text AS paid_amount,
       s.total_amount::text, s.currency, s.status, s.created_at, s.updated_at, s.service_config_snapshot
FROM services s
LEFT JOIN milestones m ON m.service_id = s.id
WHERE s.shop_id = $1
GROUP BY s.id, s.display_id, s.shop_id, s.shopify_order_id, s.shopify_product_id, s.client_email, s.client_name,
         s.total_amount, s.currency, s.status, s.created_at, s.updated_at, s.service_config_snapshot
ORDER BY s.created_at DESC
`
	rows, err := r.db.Query(ctx, q, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ListItem
	for rows.Next() {
		var s ListItem
		if err := rows.Scan(
			&s.ID, &s.DisplayID, &s.ShopID, &s.ShopifyOrderID, &s.ShopifyProductID, &s.ClientEmail, &s.ClientName,
			&s.PaidAmount, &s.TotalAmount, &s.Currency, &s.Status, &s.CreatedAt, &s.UpdatedAt, &s.ServiceConfigSnapshot,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, shopID, serviceID string) (*Service, error) {
	const q = `
SELECT id, display_id, shop_id, shopify_order_id, shopify_product_id, client_email, client_name,
       total_amount::text, currency, status, service_config_snapshot, completed_via_override,
       created_at, updated_at
FROM services
WHERE shop_id = $1 AND id = $2
`
	var s Service
	if err := r.db.QueryRow(ctx, q, shopID, serviceID).Scan(
		&s.ID, &s.DisplayID, &s.ShopID, &s.ShopifyOrderID, &s.ShopifyProductID, &s.ClientEmail, &s.ClientName,
		&s.TotalAmount, &s.Currency, &s.Status, &s.ServiceConfigSnapshot, &s.CompletedViaOverride,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

func GetForUpdate(ctx context.Context, tx pgx.Tx, shopID, serviceID string) (*Service, error) {
	const q = `
SELECT id, display_id, shop_id, shopify_order_id, shopify_product_id, client_email, client_name,
       total_amount::text, currency, status, service_config_snapshot, completed_via_override,
       created_at, updated_at
FROM services
WHERE shop_id = $1 AND id = $2
FOR UPDATE
`
	var s Service
	if err := tx.QueryRow(ctx, q, shopID, serviceID).Scan(
		&s.ID, &s.DisplayID, &s.ShopID, &s.ShopifyOrderID, &s.ShopifyProductID, &s.ClientEmail, &s.ClientName,
		&s.TotalAmount, &s.Currency, &s.Status, &s.ServiceConfigSnapshot, &s.CompletedViaOverride,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

func GetForUpdateAny(ctx context.Context, tx pgx.Tx, serviceID string) (*Service, error) {
	const q = `
SELECT id, display_id, shop_id, shopify_order_id, shopify_product_id, client_email, client_name,
       total_amount::text, currency, status, service_config_snapshot, completed_via_override,
       created_at, updated_at
FROM services
WHERE id = $1
FOR UPDATE
`
	var s Service
	if err := tx.QueryRow(ctx, q, serviceID).Scan(
		&s.ID, &s.DisplayID, &s.ShopID, &s.ShopifyOrderID, &s.ShopifyProductID, &s.ClientEmail, &s.ClientName,
		&s.TotalAmount, &s.Currency, &s.Status, &s.ServiceConfigSnapshot, &s.CompletedViaOverride,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

func UpdateStatus(ctx context.Context, tx pgx.Tx, shopID, serviceID string, next Status, completedViaOverride bool) error {
	const q = `
UPDATE services
SET status = $1, completed_via_override = $2, updated_at = NOW()
WHERE shop_id = $3 AND id = $4
`
	_, err := tx.Exec(ctx, q, string(next), completedViaOverride, shopID, serviceID)
	return err
}



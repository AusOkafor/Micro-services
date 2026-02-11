package milestone

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Record struct {
	ID          string     `json:"id"`
	ServiceID   string     `json:"serviceId"`
	Sequence    int        `json:"sequence"`
	Amount      string     `json:"amount"`
	Status      string     `json:"status"`
	Currency    string     `json:"currency,omitempty"`
	DraftOrderID string    `json:"draftOrderId,omitempty"`
	CheckoutURL string     `json:"checkoutUrl,omitempty"`
	PaidAt      *time.Time `json:"paidAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByService(ctx context.Context, serviceID string) ([]Record, error) {
	const q = `
SELECT id, service_id, sequence, amount::text, status, draft_order_id, checkout_url, paid_at, created_at
FROM milestones
WHERE service_id = $1
ORDER BY sequence ASC
`
	rows, err := r.db.Query(ctx, q, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var rec Record
		var draftOrderID, checkoutURL *string
		if err := rows.Scan(&rec.ID, &rec.ServiceID, &rec.Sequence, &rec.Amount, &rec.Status, &draftOrderID, &checkoutURL, &rec.PaidAt, &rec.CreatedAt); err != nil {
			return nil, err
		}
		if draftOrderID != nil {
			rec.DraftOrderID = *draftOrderID
		}
		if checkoutURL != nil {
			rec.CheckoutURL = *checkoutURL
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func GetForUpdate(ctx context.Context, tx pgx.Tx, milestoneID string) (*Record, error) {
	const q = `
SELECT id, service_id, sequence, amount::text, status, draft_order_id, checkout_url, paid_at, created_at
FROM milestones
WHERE id = $1
FOR UPDATE
`
	var rec Record
	var draftOrderID, checkoutURL *string
	if err := tx.QueryRow(ctx, q, milestoneID).Scan(
		&rec.ID, &rec.ServiceID, &rec.Sequence, &rec.Amount, &rec.Status, &draftOrderID, &checkoutURL, &rec.PaidAt, &rec.CreatedAt,
	); err != nil {
		return nil, err
	}
	if draftOrderID != nil {
		rec.DraftOrderID = *draftOrderID
	}
	if checkoutURL != nil {
		rec.CheckoutURL = *checkoutURL
	}
	return &rec, nil
}

func GetForUpdateScoped(ctx context.Context, tx pgx.Tx, shopID string, milestoneID string) (*Record, error) {
	const q = `
SELECT m.id, m.service_id, m.sequence, m.amount::text, m.status, s.currency, m.draft_order_id, m.checkout_url, m.paid_at, m.created_at
FROM milestones m
JOIN services s ON s.id = m.service_id
WHERE m.id = $1 AND s.shop_id = $2
FOR UPDATE OF m
`
	var rec Record
	var draftOrderID, checkoutURL *string
	if err := tx.QueryRow(ctx, q, milestoneID, shopID).Scan(
		&rec.ID, &rec.ServiceID, &rec.Sequence, &rec.Amount, &rec.Status, &rec.Currency, &draftOrderID, &checkoutURL, &rec.PaidAt, &rec.CreatedAt,
	); err != nil {
		return nil, err
	}
	if draftOrderID != nil {
		rec.DraftOrderID = *draftOrderID
	}
	if checkoutURL != nil {
		rec.CheckoutURL = *checkoutURL
	}
	return &rec, nil
}

func SetDraftOrder(ctx context.Context, tx pgx.Tx, milestoneID string, draftOrderID string, checkoutURL string) error {
	const q = `
UPDATE milestones
SET draft_order_id = $2,
    checkout_url = $3
WHERE id = $1
`
	_, err := tx.Exec(ctx, q, milestoneID, draftOrderID, checkoutURL)
	return err
}

func UnlockFinal(ctx context.Context, tx pgx.Tx, serviceID string) error {
	const q = `
UPDATE milestones
SET status = 'unpaid'
WHERE service_id = $1
  AND sequence = (SELECT sequence FROM milestones WHERE service_id = $1 ORDER BY sequence DESC LIMIT 1)
  AND status = 'locked'
`
	_, err := tx.Exec(ctx, q, serviceID)
	return err
}

func MarkPaid(ctx context.Context, tx pgx.Tx, milestoneID string, paidAt time.Time) error {
	const q = `
UPDATE milestones
SET status = 'paid', paid_at = $2
WHERE id = $1
`
	_, err := tx.Exec(ctx, q, milestoneID, paidAt)
	return err
}



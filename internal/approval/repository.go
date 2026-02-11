package approval

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Record struct {
	ServiceID         string  `json:"serviceId"`
	Approved          bool    `json:"approved"`
	RevisionRequested bool    `json:"revisionRequested"`
	ClientNote        string  `json:"clientNote,omitempty"`
	HasPreview        bool    `json:"hasPreview"`
	ApprovedAt        *string `json:"approvedAt,omitempty"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetByService(ctx context.Context, serviceID string) (*Record, error) {
	const q = `
SELECT service_id, approved, revision_requested, COALESCE(client_note,''), has_preview,
       CASE WHEN approved_at IS NULL THEN NULL ELSE approved_at::text END
FROM approvals
WHERE service_id = $1
`
	var rec Record
	if err := r.db.QueryRow(ctx, q, serviceID).Scan(
		&rec.ServiceID, &rec.Approved, &rec.RevisionRequested, &rec.ClientNote, &rec.HasPreview, &rec.ApprovedAt,
	); err != nil {
		return nil, err
	}
	return &rec, nil
}

func UpsertRequested(ctx context.Context, tx pgx.Tx, serviceID string, hasPreview bool) error {
	const q = `
INSERT INTO approvals (service_id, has_preview)
VALUES ($1, $2)
ON CONFLICT (service_id) DO UPDATE SET
  has_preview = EXCLUDED.has_preview,
  updated_at = NOW()
`
	_, err := tx.Exec(ctx, q, serviceID, hasPreview)
	return err
}

func Approve(ctx context.Context, tx pgx.Tx, serviceID string, note string) error {
	const q = `
UPDATE approvals
SET approved = TRUE,
    approved_at = NOW(),
    revision_requested = FALSE,
    client_note = $2,
    updated_at = NOW()
WHERE service_id = $1
`
	_, err := tx.Exec(ctx, q, serviceID, note)
	return err
}

func RequestRevision(ctx context.Context, tx pgx.Tx, serviceID string, note string) error {
	const q = `
UPDATE approvals
SET revision_requested = TRUE,
    approved = FALSE,
    approved_at = NULL,
    client_note = $2,
    updated_at = NOW()
WHERE service_id = $1
`
	_, err := tx.Exec(ctx, q, serviceID, note)
	return err
}

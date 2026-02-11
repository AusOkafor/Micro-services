package files

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Record struct {
	ID         string    `json:"id"`
	ServiceID  string    `json:"serviceId"`
	UploadedBy string    `json:"uploadedBy"` // merchant | client
	FileURL    string    `json:"fileUrl"`
	Kind       string    `json:"kind"` // preview | deliverable | other
	CreatedAt  time.Time `json:"createdAt"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(ctx context.Context, serviceID, uploadedBy, fileURL, kind string) (*Record, error) {
	const q = `
INSERT INTO files (service_id, uploaded_by, file_url, kind)
VALUES ($1, $2, $3, $4)
RETURNING id, service_id, uploaded_by, file_url, kind, created_at
`
	var rec Record
	if err := r.db.QueryRow(ctx, q, serviceID, uploadedBy, fileURL, kind).Scan(
		&rec.ID, &rec.ServiceID, &rec.UploadedBy, &rec.FileURL, &rec.Kind, &rec.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *Repository) ListByService(ctx context.Context, serviceID string) ([]Record, error) {
	const q = `
SELECT id, service_id, uploaded_by, file_url, kind, created_at
FROM files
WHERE service_id = $1
ORDER BY created_at ASC
`
	rows, err := r.db.Query(ctx, q, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.ID, &rec.ServiceID, &rec.UploadedBy, &rec.FileURL, &rec.Kind, &rec.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}



package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func Insert(ctx context.Context, tx pgx.Tx, shopID string, serviceID *string, action, actor string, metadata any) error {
	var s *string
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		str := string(b)
		s = &str
	}
	const q = `
INSERT INTO audit_logs (shop_id, service_id, action, actor, metadata)
VALUES ($1, $2, $3, $4, CAST($5 AS jsonb))
`
	_, err := tx.Exec(ctx, q, shopID, serviceID, action, actor, s)
	return err
}



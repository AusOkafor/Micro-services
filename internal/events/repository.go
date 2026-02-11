package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func Insert(ctx context.Context, tx pgx.Tx, serviceID, eventType, summary, actor string, occurredAt time.Time, data any) error {
	var s *string
	if data != nil {
		b, _ := json.Marshal(data)
		str := string(b)
		s = &str
	}
	const q = `
INSERT INTO service_events (service_id, event_type, summary, actor, occurred_at, data)
VALUES ($1, $2, $3, $4, $5, CAST($6 AS jsonb))
`
	_, err := tx.Exec(ctx, q, serviceID, eventType, summary, actor, occurredAt, s)
	return err
}



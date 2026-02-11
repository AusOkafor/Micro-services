package portal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenRecord struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"serviceId"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetActiveByService(ctx context.Context, serviceID string, now time.Time) (*TokenRecord, error) {
	const q = `
SELECT id, service_id, token, expires_at, revoked_at, created_at
FROM portal_tokens
WHERE service_id = $1
  AND revoked_at IS NULL
  AND expires_at > $2
ORDER BY created_at DESC
LIMIT 1
`
	var tr TokenRecord
	if err := r.db.QueryRow(ctx, q, serviceID, now).Scan(&tr.ID, &tr.ServiceID, &tr.Token, &tr.ExpiresAt, &tr.RevokedAt, &tr.CreatedAt); err != nil {
		return nil, err
	}
	return &tr, nil
}

func GetActiveByTokenForUpdate(ctx context.Context, tx pgx.Tx, token string, now time.Time) (*TokenRecord, error) {
	const q = `
SELECT id, service_id, token, expires_at, revoked_at, created_at
FROM portal_tokens
WHERE token = $1
FOR UPDATE
`
	var tr TokenRecord
	if err := tx.QueryRow(ctx, q, token).Scan(&tr.ID, &tr.ServiceID, &tr.Token, &tr.ExpiresAt, &tr.RevokedAt, &tr.CreatedAt); err != nil {
		return nil, err
	}
	if tr.RevokedAt != nil || !tr.ExpiresAt.After(now) {
		return nil, pgx.ErrNoRows
	}
	return &tr, nil
}

func InsertToken(ctx context.Context, tx pgx.Tx, serviceID string, expiresAt time.Time) (*TokenRecord, error) {
	token := randomHex(32)
	const q = `
INSERT INTO portal_tokens (service_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING id, service_id, token, expires_at, revoked_at, created_at
`
	var tr TokenRecord
	if err := tx.QueryRow(ctx, q, serviceID, token, expiresAt).Scan(&tr.ID, &tr.ServiceID, &tr.Token, &tr.ExpiresAt, &tr.RevokedAt, &tr.CreatedAt); err != nil {
		return nil, err
	}
	return &tr, nil
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}



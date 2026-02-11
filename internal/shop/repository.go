package shop

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Upsert(ctx context.Context, domain, accessToken string) (*Shop, error) {
	const q = `
INSERT INTO shops (shop_domain, access_token, status)
VALUES ($1, $2, 'active')
ON CONFLICT (shop_domain) DO UPDATE SET
  access_token = EXCLUDED.access_token,
  status = 'active'
RETURNING id, shop_domain, access_token, COALESCE(plan,''), COALESCE(status,'active'), installed_at
`
	s := &Shop{}
	if err := r.db.QueryRow(ctx, q, domain, accessToken).Scan(
		&s.ID, &s.Domain, &s.AccessToken, &s.Plan, &s.Status, &s.InstalledAt,
	); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) FindByDomain(ctx context.Context, domain string) (*Shop, error) {
	const q = `
SELECT id, shop_domain, access_token, COALESCE(plan,''), COALESCE(status,'active'), installed_at
FROM shops
WHERE shop_domain = $1
`
	s := &Shop{}
	if err := r.db.QueryRow(ctx, q, domain).Scan(
		&s.ID, &s.Domain, &s.AccessToken, &s.Plan, &s.Status, &s.InstalledAt,
	); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) DeleteByID(ctx context.Context, id string) error {
	const q = `DELETE FROM shops WHERE id = $1`
	_, err := r.db.Exec(ctx, q, id)
	return err
}



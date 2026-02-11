package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"microservice/pkg/config"
)

func Open(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	connString := runtimeConnString(cfg)

	// Supabase pooler (PgBouncer) does not support prepared statements.
	// Their pooler DSN typically includes `pgbouncer=true`.
	pcfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(connString), "pgbouncer=true") {
		pcfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		pcfg.ConnConfig.StatementCacheCapacity = 0
		pcfg.ConnConfig.DescriptionCacheCapacity = 0
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func runtimeConnString(cfg config.Config) string {
	if strings.TrimSpace(cfg.DatabaseURL) != "" {
		return cfg.DatabaseURL
	}
	return dsn(cfg.DB)
}

func migrationConnString(cfg config.Config) string {
	if strings.TrimSpace(cfg.DirectURL) != "" {
		return cfg.DirectURL
	}
	// Fall back to runtime conn string; works for local dev, but for Supabase
	// migrations should use DIRECT_URL to avoid PgBouncer limitations.
	return runtimeConnString(cfg)
}

func dsn(cfg config.DBConfig) string {
	sslmode := cfg.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, sslmode,
	)
}



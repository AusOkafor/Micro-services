package db

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"microservice/pkg/config"
)

func Migrate(migrationsPath string, cfg config.DBConfig) error {
	// Backwards-compatible wrapper; prefer MigrateConfig.
	return MigrateConfig(migrationsPath, config.Config{DB: cfg})
}

func MigrateConfig(migrationsPath string, cfg config.Config) error {
	m, err := migrate.New(migrationsPath, migrationConnString(cfg))
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return err
	}
	return nil
}



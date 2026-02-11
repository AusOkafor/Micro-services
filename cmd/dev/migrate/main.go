package main

import (
	"context"
	"fmt"
	"os"

	"microservice/pkg/config"
	"microservice/pkg/db"
)

func main() {
	cfg := config.Load()
	if cfg.MigrationsPath == "" {
		cfg.MigrationsPath = "file://migrations"
	}

	// This uses DIRECT_URL if set (recommended for Supabase migrations).
	if err := db.MigrateConfig(cfg.MigrationsPath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "migrate failed: %v\n", err)
		os.Exit(1)
	}

	// Optional sanity check: ensure runtime connection can open (uses DATABASE_URL if set).
	// We don't print DSNs here to avoid leaking secrets into logs.
	pool, err := db.Open(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "runtime db open failed: %v\n", err)
		os.Exit(1)
	}
	pool.Close()

	fmt.Println("migrations applied")
}



package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"microservice/internal/httpapi"
	"microservice/pkg/config"
	"microservice/pkg/db"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer conn.Close()

	if cfg.MigrationsPath != "" {
		if err := db.MigrateConfig(cfg.MigrationsPath, cfg); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	}

	router := httpapi.NewRouter(httpapi.Dependencies{
		Cfg: cfg,
		DB:  conn,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("http listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http serve: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = srv.Shutdown(shutdownCtx)
}



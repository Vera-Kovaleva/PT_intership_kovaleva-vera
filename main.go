package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"urlshortener/internal/config"
	"urlshortener/internal/httpapi"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
	"urlshortener/migrations"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := applyMigrations(cfg.DBConnection); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DBConnection)
	if err != nil {
		log.Fatalf("database pool: %v", err)
	}
	defer pool.Close()

	repo := repository.NewPostgres(pool)
	svc := service.New(repo)
	h := httpapi.NewHandler(svc, cfg.BaseURL)

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           h.Routes(),
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
	}

	go func() {
		log.Printf("listening and serve on: %v", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listening and serve error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down server...")

	shutDownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutDownCtx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}

func applyMigrations(dsn string) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return err
	}
	db := stdlib.OpenDB(*poolCfg.ConnConfig)
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

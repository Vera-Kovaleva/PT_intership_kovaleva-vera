package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"urlshortener/internal/cache"
	"urlshortener/internal/config"
	"urlshortener/internal/handler"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
	"urlshortener/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := migrations.Apply(cfg.DBConnection); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	logger.Info("migrations applied")

	pool, err := pgxpool.New(ctx, cfg.DBConnection)
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	defer pool.Close()

	repo := repository.NewPostgres(pool)
	var linksCache cache.Cache = cache.Noop{}
	if cfg.RedisAddress != "" {
		client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddress})
		defer func() { _ = client.Close() }()
		linksCache = cache.NewRedis(client, logger)
	} else {
		logger.Warn("running without cache", "reason", "REDIS_ADDRESS is not set")
	}
	svc := service.New(repo, linksCache, logger)
	h := handler.NewHandler(svc, cfg.BaseURL, logger)

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           h.Handler(logger, config.ReadTimeout),
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutDownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutDownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("server stopped")
	return nil
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(contextHandler{slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})})
}

type contextHandler struct {
	slog.Handler
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := handler.RequestIDFrom(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

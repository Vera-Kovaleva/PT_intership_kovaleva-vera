package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"urlshortener/internal/config"
	"urlshortener/internal/httpapi"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
)

func main() {

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	repo := repository.NewMemory()
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

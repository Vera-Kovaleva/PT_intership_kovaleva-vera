package config_test

import (
	"testing"
	"urlshortener/internal/config"
)

func TestLoadRequiresVars(t *testing.T) {
	t.Setenv("BASE_URL", "")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected an error when requred variables are unset")
	}
}

func TestLoadReadsValues(t *testing.T) {
	t.Setenv("BASE_URL", "http://example.com")
	t.Setenv("DB_CONNECTION", "postgres://x")
	t.Setenv("SERVER_PORT", "9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "9090")
	}
}

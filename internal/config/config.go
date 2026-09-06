package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ReadHeaderTimeout = 5 * time.Second
	ReadTimeout       = 5 * time.Second
	WriteTimeout      = 5 * time.Second
	IdleTimeout       = 60 * time.Second
	ShutdownTimeout   = 10 * time.Second
	RequestTimeout    = 3 * time.Second
	DBQueryTimeout    = 2 * time.Second

	MaxBodyBytes = 16384
	MaxURLLength = 2048

	RedisTimeout     = 200 * time.Millisecond
	RedisTTL         = 24 * time.Hour
	RedisNegativeTTL = 1 * time.Minute
)

type Config struct {
	ServerPort   string
	DBConnection string
	RedisAddress string
	BaseURL      string
	LogLevel     string
}

func Load() (Config, error) {
	var c Config

	c.ServerPort = env("SERVER_PORT", "8080")
	c.LogLevel = env("LOG_LEVEL", "info")
	c.BaseURL = os.Getenv("BASE_URL")
	c.DBConnection = os.Getenv("DB_CONNECTION")
	c.RedisAddress = os.Getenv("REDIS_ADDRESS")

	var missing []string
	if c.BaseURL == "" {
		missing = append(missing, "BASE_URL")
	}
	if c.DBConnection == "" {
		missing = append(missing, "DB_CONNECTION")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("required environment variables are not set: %s", strings.Join(missing, ", "))
	}

	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

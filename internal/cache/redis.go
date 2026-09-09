package cache

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"
	"urlshortener/internal/config"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client      *redis.Client
	ttl         time.Duration
	negativeTTL time.Duration
	timeout     time.Duration
	logger      *slog.Logger
}

func NewRedis(client *redis.Client, logger *slog.Logger) *Redis {
	return &Redis{
		client:      client,
		ttl:         config.RedisTTL,
		negativeTTL: config.RedisNegativeTTL,
		timeout:     config.RedisTimeout,
		logger:      logger,
	}
}

var _ Cache = (*Redis)(nil)

func (r *Redis) Get(ctx context.Context, code string) (string, Result) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	v, err := r.client.Get(ctx, key(code)).Result()
	switch {
	case errors.Is(err, redis.Nil):
		return "", Miss
	case err != nil:
		r.logger.WarnContext(ctx, "cache get failed", "error", err)
		return "", Miss
	case v == "":
		return "", HitNegative
	}
	return v, Hit
}

func (r *Redis) Set(ctx context.Context, code, originalURL string) {
	r.set(ctx, code, originalURL, r.ttl)
}

func (r *Redis) SetNegative(ctx context.Context, code string) {
	r.set(ctx, code, "", r.negativeTTL)
}

func (r *Redis) set(ctx context.Context, code, value string, ttl time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if err := r.client.Set(ctx, key(code), value, ttl+rand.N(ttl/10)).Err(); err != nil {
		r.logger.WarnContext(ctx, "cache set failed", "error", err)
	}
}

func key(code string) string { return "url:" + code }

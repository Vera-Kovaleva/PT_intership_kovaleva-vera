package cache

import (
	"context"
	"log/slog"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestRedisTreatsFailureAsMiss(t *testing.T) {
	redis.SetLogger(discardLogger{})

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = client.Close() })

	c := NewRedis(client, slog.New(slog.DiscardHandler))
	ctx := context.Background()

	url, res := c.Get(ctx, "aaaaaa")
	if res != Miss {
		t.Fatalf("result: got %v, want %v", res, Miss)
	}
	if url != "" {
		t.Fatalf("url: got %q, want %q", url, "")
	}

	c.Set(ctx, "aaaaaa", "https://example.com")
	c.SetNegative(ctx, "aaaaaa")
}

type discardLogger struct{}

func (discardLogger) Printf(context.Context, string, ...any) {}

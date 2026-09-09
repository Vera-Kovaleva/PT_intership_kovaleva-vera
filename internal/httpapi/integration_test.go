//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"urlshortener/internal/cache"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
	"urlshortener/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestShortenAndResolveAgainstPostgres(t *testing.T) {
	handler, _ := newIntegrationHandler(t)
	const target = "https://integration.example/page"

	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"`+target+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("shorten status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		ShortURL string `json:"short_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	code := strings.TrimPrefix(resp.ShortURL, "http://localhost:8080/")

	req = httptest.NewRequest(http.MethodGet, "/"+code, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("redirect status: got %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != target {
		t.Fatalf("Location: got %q, want %q", got, target)
	}
}

func TestConcurrentShortenCreatesSingleRow(t *testing.T) {
	handler, pool := newIntegrationHandler(t)
	const n = 20
	const target = "https://race.example/page"

	urls := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := range n {
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"`+target+`"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
				return
			}
			var resp struct {
				ShortURL string `json:"short_url"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Errorf("decode: %v", err)
				return
			}
			urls[i] = resp.ShortURL
		}()
	}
	wg.Wait()

	for i, u := range urls {
		if u != urls[0] {
			t.Fatalf("short_url %d: got %q, want %q", i, u, urls[0])
		}
	}

	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM urls").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows: got %d, want 1", count)
	}
}

func newIntegrationHandler(t *testing.T) (http.Handler, *pgxpool.Pool) {
	t.Helper()

	dsn := os.Getenv("TEST_DB_CONNECTION")
	if dsn == "" {
		t.Skip("TEST_DB_CONNECTION is not set")
	}
	if err := migrations.Apply(dsn); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(context.Background(), "TRUNCATE urls"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	logger := slog.New(slog.DiscardHandler)
	svc := service.New(repository.NewPostgres(pool), cache.Noop{}, logger)
	h := NewHandler(svc, "http://localhost:8080", logger)
	return h.Handler(logger, time.Second), pool
}

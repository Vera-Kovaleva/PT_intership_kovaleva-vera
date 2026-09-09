package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
)

func TestRedirectsToOriginalURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	rec := httptest.NewRecorder()

	stub := stubService{
		resolve: func(context.Context, string) (string, error) {
			return "https://example.com", nil
		},
	}

	newTestHandler(t, stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != "https://example.com" {
		t.Fatalf("Location: got %q, want %q", got, "https://example.com")
	}
}

func TestReturnsNotFoundForUnknownCode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/abc123", nil)
	rec := httptest.NewRecorder()

	stub := stubService{
		resolve: func(context.Context, string) (string, error) {
			return "", repository.ErrNotFound
		},
	}

	newTestHandler(t, stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("Location: got %q, want %q", got, "")
	}
}

func TestShortenReturnsUnprocessableForInvalidURL(t *testing.T) {
	stub := stubService{
		shorten: func(context.Context, string) (string, error) { return "", service.ErrInvalidURL },
	}

	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"ftp://example.com"}`))
	rec := httptest.NewRecorder()
	newTestHandler(t, stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestRejectsWrongMethodWithAllow(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/shorten", nil)
	rec := httptest.NewRecorder()

	stub := stubService{}

	newTestHandler(t, stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("Allow: got %q, want %q", got, http.MethodPost)
	}
}

func TestHealthReportsDegradedWhenPingFails(t *testing.T) {
	stub := stubService{
		ping: func(context.Context) error { return errors.New("db is down") },
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	newTestHandler(t, stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "degraded" {
		t.Fatalf("status field: got %q, want %q", body.Status, "degraded")
	}
}

func TestShortenAcceptsRequestWithoutContentType(t *testing.T) {
	stub := stubService{
		shorten: func(context.Context, string) (string, error) { return "abc123", nil },
	}

	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	rec := httptest.NewRecorder()
	newTestHandler(t, stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		ShortURL string `json:"short_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ShortURL != "http://localhost:8080/abc123" {
		t.Fatalf("short_url: got %q, want %q", resp.ShortURL, "http://localhost:8080/abc123")
	}
}

func TestShortenRejectsBadRequests(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{"rejects non-json content type", "text/plain", `{"url":"https://a.com"}`, http.StatusUnsupportedMediaType},
		{"rejects broken json", "application/json", `{`, http.StatusBadRequest},
		{"rejects json array", "application/json", `[1,2,3]`, http.StatusBadRequest},
		{"rejects url of wrong type", "application/json", `{"url":123}`, http.StatusUnprocessableEntity},
		{"rejects missing url field", "application/json", `{}`, http.StatusUnprocessableEntity},
		{"rejects body over the size limit", "application/json", `{"url":"https://a.com` + strings.Repeat("a", 20000) + `"}`, http.StatusRequestEntityTooLarge},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(c.body))
			req.Header.Set("Content-Type", c.contentType)
			rec := httptest.NewRecorder()

			stub := stubService{}

			newTestHandler(t, stub).ServeHTTP(rec, req)

			if rec.Code != c.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, c.wantStatus)
			}
		})
	}
}

type stubService struct {
	shorten func(context.Context, string) (string, error)
	resolve func(context.Context, string) (string, error)
	ping    func(context.Context) error
}

func (s stubService) Shorten(ctx context.Context, url string) (string, error) {
	if s.shorten == nil {
		return "", nil
	}
	return s.shorten(ctx, url)
}

func (s stubService) Resolve(ctx context.Context, url string) (string, error) {
	if s.resolve == nil {
		return "", nil
	}
	return s.resolve(ctx, url)
}

func (s stubService) Ping(ctx context.Context) error {
	if s.ping == nil {
		return nil
	}
	return s.ping(ctx)
}

func newTestHandler(t *testing.T, stub stubService) http.Handler {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	h := NewHandler(stub, "http://localhost:8080", logger)
	return h.Handler(logger, time.Second)
}

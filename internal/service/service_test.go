package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"urlshortener/internal/cache"
	"urlshortener/internal/repository"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	return New(repository.NewMemory(), cache.Noop{}, slog.New(slog.DiscardHandler))
}

func TestShortenReturnsSameCodeForSameURL(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	first, err := svc.Shorten(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("first shorten: %v", err)
	}

	second, err := svc.Shorten(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("second shorten: %v", err)
	}

	if first != second {
		t.Fatalf("code: got %q on second call, want %q", second, first)
	}

	got, err := svc.Resolve(ctx, first)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("original url: got %q, want %q", got, "https://example.com")
	}
}

func TestShortenNormalizesURL(t *testing.T) {
	cases := []struct {
		name     string
		a, b     string
		wantSame bool
	}{
		{"ignores scheme and host case", "https://example.com", "HTTPS://Example.COM", true},
		{"treats trailing slash as different url", "https://example.com", "https://example.com/", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := newTestService(t)
			ctx := context.Background()

			first, err := svc.Shorten(ctx, c.a)
			if err != nil {
				t.Fatalf("first shorten: %v", err)
			}

			second, err := svc.Shorten(ctx, c.b)
			if err != nil {
				t.Fatalf("second shorten: %v", err)
			}

			if (first == second) != c.wantSame {
				t.Fatalf("code: got %v on second call, want %v", first != second, c.wantSame)
			}
		})
	}
}

func TestShortenRetriesOnCodeCollision(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	codes := []string{"aaaaaa", "aaaaaa", "bbbbbb"}
	i := 0
	svc.newCode = func() (string, error) {
		code := codes[i]
		i++
		return code, nil
	}

	first, err := svc.Shorten(ctx, "https://one.example")
	if err != nil {
		t.Fatalf("first shorten: %v", err)
	}
	if first != "aaaaaa" {
		t.Fatalf("first code: got %q, want %q", first, "aaaaaa")
	}

	second, err := svc.Shorten(ctx, "https://two.example")
	if err != nil {
		t.Fatalf("second shorten: %v", err)
	}
	if second != "bbbbbb" {
		t.Fatalf("second code: got %q, want %q", second, "bbbbbb")
	}
}

func TestShortenFailsWhenCodeSpaceExhausted(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.newCode = func() (string, error) { return "aaaaaa", nil }

	if _, err := svc.Shorten(ctx, "https://one.example"); err != nil {
		t.Fatalf("first shorten: %v", err)
	}

	_, err := svc.Shorten(ctx, "https://two.example")
	if !errors.Is(err, ErrCodeSpaceExhausted) {
		t.Fatalf("error: got %v, want %v", err, ErrCodeSpaceExhausted)
	}
}

type countingRepo struct {
	repository.Repository
	findByCode atomic.Int64
}

func (r *countingRepo) FindByCode(ctx context.Context, code string) (repository.Link, error) {
	r.findByCode.Add(1)
	return r.Repository.FindByCode(ctx, code)
}

type hitCache struct {
	cache.Noop
	url string
}

func (c hitCache) Get(context.Context, string) (string, cache.Result) { return c.url, cache.Hit }

func TestResolveSkipsRepositoryOnCacheHit(t *testing.T) {
	repo := &countingRepo{Repository: repository.NewMemory()}
	svc := New(repo, hitCache{url: "https://example.com"}, slog.New(slog.DiscardHandler))

	got, err := svc.Resolve(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("url: got %q, want %q", got, "https://example.com")
	}
	if n := repo.findByCode.Load(); n != 0 {
		t.Fatalf("repository calls: got %d, want 0", n)
	}
}

type blockingRepo struct {
	repository.Repository
	calls   atomic.Int64
	release chan struct{}
}

func (r *blockingRepo) FindByCode(ctx context.Context, code string) (repository.Link, error) {
	r.calls.Add(1)
	<-r.release
	return repository.Link{ShortCode: code, OriginalURL: "https://example.com"}, nil
}

func TestResolveCollapsesConcurrentMisses(t *testing.T) {
	repo := &blockingRepo{Repository: repository.NewMemory(), release: make(chan struct{})}
	svc := New(repo, cache.Noop{}, slog.New(slog.DiscardHandler))

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			if _, err := svc.Resolve(context.Background(), "abc123"); err != nil {
				t.Errorf("resolve: %v", err)
			}
		}()
	}

	time.Sleep(100 * time.Millisecond) // даём горутинам дойти до singleflight
	close(repo.release)
	wg.Wait()

	if got := repo.calls.Load(); got != 1 {
		t.Fatalf("repository calls: got %d, want 1", got)
	}
}

func TestShortenIsSafeForConcurrentCalls(t *testing.T) {
	svc := newTestService(t)
	const n = 50

	codes := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := range n {
		go func() {
			defer wg.Done()
			code, err := svc.Shorten(context.Background(), "https://race.example")
			if err != nil {
				t.Errorf("shorten: %v", err)
				return
			}
			codes[i] = code
		}()
	}
	wg.Wait()

	for i, code := range codes {
		if code != codes[0] {
			t.Fatalf("code %d: got %q, want %q", i, code, codes[0])
		}
	}
}

package service

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"urlshortener/internal/config"
)

func TestHashIsStable(t *testing.T) {
	_, h, err := normalizeAndHash("HTTPS://Example.com/a")
	if err != nil {
		t.Fatal(err)
	}
	const want = "2dce0a4c50441bfccfa9caf4b58c3cba6e06c420505dd829f0436de1aa44baac"
	if got := hex.EncodeToString(h); got != want {
		t.Fatalf("hash changed: got %s, want %s. Stored hashes were computed by the "+
			"old rules and will stop matching without a recompute migration", got, want)
	}
}

func TestEmptyURL(t *testing.T) {
	_, _, err := normalizeAndHash("")
	if err == nil {
		t.Fatal("expected an error for an empty url")
	}
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("errors.Is(err, ErrInvalidURL) = false, err = %v", err)
	}
}

func TestValidateRejectsBadURLs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"rejects javascript scheme", "javascript:alert(1)"},
		{"rejects javascript scheme disguised as host", "javascript://example.com/%0aalert(1)"},
		{"rejects ftp scheme", "ftp://example.com"},
		{"rejects relative path", "/foo"},
		{"rejects url with userinfo", "https://a.com@evil.example"},
		{"rejects url over the length limit", "https://example.com/" + strings.Repeat("a", config.MaxURLLength)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := normalizeAndHash(c.raw); !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("error: got %v, want %v", err, ErrInvalidURL)
			}
		})
	}
}

package service

import (
	"encoding/hex"
	"errors"
	"testing"
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

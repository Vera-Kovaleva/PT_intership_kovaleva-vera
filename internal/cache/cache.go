package cache

import "context"

type Result int

const (
	Miss Result = iota
	Hit
	HitNegative
)

type Cache interface {
	Get(ctx context.Context, code string) (string, Result)
	Set(ctx context.Context, code, originalURL string)
	SetNegative(ctx context.Context, code string)
}

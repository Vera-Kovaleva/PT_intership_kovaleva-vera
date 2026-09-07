package repository

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("link not found")

type Link struct {
	ShortCode   string
	OriginalURL string
	URLHash     []byte
}

type Repository interface {
	FindByHash(context.Context, []byte) (Link, error)
	FindByCode(context.Context, string) (Link, error)
	Insert(context.Context, Link) error
	Ping(context.Context) error
}

type ConflictError struct {
	Constraint string
}

func (e *ConflictError) Error() string {
	return "unique violation on " + e.Constraint
}

const (
	ConstraintURLHash   = "urls_url_hash_key"
	ConstraintShortCode = "urls_pkey"
)

type UnavailableError struct {
	Err error
}

func (e *UnavailableError) Error() string { return "storage unavailable: " + e.Err.Error() }
func (e *UnavailableError) Unwrap() error { return e.Err }

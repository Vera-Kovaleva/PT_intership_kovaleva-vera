package repository

import (
	"context"
	"errors"
	"urlshortener/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

const uniqueViolation = "23505"

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) FindByCode(ctx context.Context, code string) (Link, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DBQueryTimeout)
	defer cancel()

	const q = `SELECT short_code, original_url, url_hash FROM urls WHERE short_code = $1`
	return p.findLink(ctx, q, code)
}

func (p *Postgres) FindByHash(ctx context.Context, hash []byte) (Link, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DBQueryTimeout)
	defer cancel()

	const q = `SELECT short_code, original_url, url_hash FROM urls WHERE url_hash = $1`
	return p.findLink(ctx, q, hash)
}

func (p *Postgres) findLink(ctx context.Context, query string, arg any) (Link, error) {
	var link Link
	err := p.pool.QueryRow(ctx, query, arg).Scan(&link.ShortCode, &link.OriginalURL, &link.URLHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, wrapDBError(err)
	}
	return link, nil
}

func wrapDBError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &UnavailableError{Err: err}
	}
	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return &UnavailableError{Err: err}
	}
	if pgconn.SafeToRetry(err) {
		return &UnavailableError{Err: err}
	}
	return err
}

func (p *Postgres) Insert(ctx context.Context, link Link) error {
	ctx, cancel := context.WithTimeout(ctx, config.DBQueryTimeout)
	defer cancel()

	const q = `INSERT INTO urls (short_code, original_url, url_hash) VALUES ($1, $2, $3)`
	_, err := p.pool.Exec(ctx, q, link.ShortCode, link.OriginalURL, link.URLHash)
	if err == nil {
		return nil
	}

	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == uniqueViolation {
		switch pgError.ConstraintName {
		case ConstraintShortCode:
			return &ConflictError{Constraint: ConstraintShortCode}
		case ConstraintURLHash:
			return &ConflictError{Constraint: ConstraintURLHash}
		}
	}
	return wrapDBError(err)
}

func (p *Postgres) Ping(ctx context.Context) error {
	if err := p.pool.Ping(ctx); err != nil {
		return &UnavailableError{Err: err}
	}
	return nil
}

var _ Repository = (*Postgres)(nil)

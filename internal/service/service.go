package service

import (
	"context"
	"errors"
	"golang.org/x/sync/singleflight"
	"log/slog"
	"urlshortener/internal/cache"
	"urlshortener/internal/repository"
)

var ErrCodeSpaceExhausted = errors.New("could not allocate a unique short code")

type Service struct {
	repo    repository.Repository
	cache   cache.Cache
	group   singleflight.Group
	newCode func() (string, error)
	logger  *slog.Logger
}

func New(repo repository.Repository, cache cache.Cache, logger *slog.Logger) *Service {
	return &Service{
		repo:    repo,
		cache:   cache,
		newCode: generateCode,
		logger:  logger,
	}
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	if !IsWellFormedCode(code) {
		return "", repository.ErrNotFound
	}

	url, res := s.cache.Get(ctx, code)
	switch res {
	case cache.Hit:
		return url, nil
	case cache.HitNegative:
		return "", repository.ErrNotFound
	}

	v, err, _ := s.group.Do(code, func() (any, error) {
		dbCtx := context.WithoutCancel(ctx)

		link, err := s.repo.FindByCode(dbCtx, code)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				s.cache.SetNegative(dbCtx, code)
			}
			return nil, err
		}

		s.cache.Set(dbCtx, code, link.OriginalURL)
		return link.OriginalURL, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (s *Service) Shorten(ctx context.Context, rawURL string) (string, error) {
	normalized, hash, err := normalizeAndHash(rawURL)
	if err != nil {
		return "", err
	}

	existing, err := s.repo.FindByHash(ctx, hash)
	if err == nil {
		s.logResult(ctx, false, existing.ShortCode, normalized)
		return existing.ShortCode, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return "", err
	}

	for range maxAttempts {
		code, err := s.newCode()
		if err != nil {
			return "", err
		}
		err = s.repo.Insert(ctx, repository.Link{ShortCode: code, OriginalURL: normalized, URLHash: hash})
		if err == nil {
			s.logResult(ctx, true, code, normalized)
			s.cache.Set(ctx, code, normalized)
			return code, nil
		}

		var confErr *repository.ConflictError
		if !errors.As(err, &confErr) {
			return "", err
		}

		switch confErr.Constraint {
		case repository.ConstraintURLHash:
			existing, findErr := s.repo.FindByHash(ctx, hash)
			if findErr != nil {
				return "", findErr
			}
			s.logResult(ctx, false, existing.ShortCode, normalized)
			return existing.ShortCode, nil

		case repository.ConstraintShortCode:
			continue
		default:
			return "", err
		}
	}
	s.logger.ErrorContext(ctx, "short code space exhausted", "attempts", maxAttempts)
	return "", ErrCodeSpaceExhausted
}

func (s *Service) logResult(ctx context.Context, created bool, code, normalized string) {
	s.logger.InfoContext(ctx, "link shortened",
		"created", created,
		"short_code", code,
		"host", targetHost(normalized),
		"url_length", len(normalized))
}

func (s *Service) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

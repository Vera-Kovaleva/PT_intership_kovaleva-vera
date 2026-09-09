package service

import (
	"context"
	"errors"
	"log/slog"
	"urlshortener/internal/repository"
)

var ErrCodeSpaceExhausted = errors.New("could not allocate a unique short code")

type Service struct {
	repo    repository.Repository
	newCode func() (string, error)
	logger  *slog.Logger
}

func New(repo repository.Repository, logger *slog.Logger) *Service {
	return &Service{repo: repo, newCode: generateCode, logger: logger}
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	if !IsWellFormedCode(code) {
		return "", repository.ErrNotFound
	}
	link, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return "", err
	}
	return link.OriginalURL, nil
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

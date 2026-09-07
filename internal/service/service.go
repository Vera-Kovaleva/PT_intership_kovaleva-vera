package service

import (
	"context"
	"errors"
	"urlshortener/internal/repository"
)

var ErrCodeSpaceExhausted = errors.New("could not allocate a unique short code")

type Service struct {
	repo    repository.Repository
	newCode func() (string, error)
}

func New(repo repository.Repository) *Service {
	return &Service{repo: repo, newCode: generateCode}
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
			return existing.ShortCode, nil

		case repository.ConstraintShortCode:
			continue
		default:
			return "", err
		}

	}
	return "", ErrCodeSpaceExhausted
}

func (s *Service) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

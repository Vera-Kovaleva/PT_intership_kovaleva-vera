package service

import (
	"crypto/sha256"
	"errors"
	"net/url"
	"strings"
	"urlshortener/internal/config"
)

var allowedSchemes = map[string]bool{"http": true, "https": true}

var ErrInvalidURL = errors.New("invalid url")

type ValidationError struct{ Public string }

func (e *ValidationError) Error() string { return e.Public }
func (e *ValidationError) Unwrap() error { return ErrInvalidURL }

func InvalidURL(public string) error { return &ValidationError{Public: public} }

func normalizeAndHash(raw string) (string, []byte, error) {
	u, err := validate(raw)
	if err != nil {
		return "", nil, err
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	normalized := u.String()
	sum := sha256.Sum256([]byte(normalized))

	return normalized, sum[:], nil
}

func validate(raw string) (*url.URL, error) {
	if len(raw) == 0 {
		return nil, InvalidURL("url must not be empty")
	}
	if len(raw) > config.MaxURLLength {
		return nil, InvalidURL("url must be at most 2048 bytes")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, InvalidURL("url could not be parsed")
	}
	if !u.IsAbs() {
		return nil, InvalidURL("url must be an absolute http or https URL")
	}
	if !allowedSchemes[strings.ToLower(u.Scheme)] {
		return nil, InvalidURL("url must be an absolute http or https URL")
	}
	if u.Host == "" {
		return nil, InvalidURL("url must have a non-empty host")
	}
	if u.User != nil {
		return nil, InvalidURL("url must not contain a username or password")
	}

	return u, nil
}

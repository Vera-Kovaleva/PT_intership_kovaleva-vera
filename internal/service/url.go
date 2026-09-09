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

type validationError struct{ Public string }

func (e *validationError) Error() string { return e.Public }
func (e *validationError) Unwrap() error { return ErrInvalidURL }

func invalidURL(public string) error { return &validationError{Public: public} }

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
		return nil, invalidURL("url must not be empty")
	}
	if len(raw) > config.MaxURLLength {
		return nil, invalidURL("url must be at most 2048 bytes")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, invalidURL("url could not be parsed")
	}
	if !u.IsAbs() {
		return nil, invalidURL("url must be an absolute http or https URL")
	}
	if !allowedSchemes[strings.ToLower(u.Scheme)] {
		return nil, invalidURL("url must be an absolute http or https URL")
	}
	if u.Host == "" {
		return nil, invalidURL("url must have a non-empty host")
	}
	if u.User != nil {
		return nil, invalidURL("url must not contain a username or password")
	}

	return u, nil
}

func targetHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

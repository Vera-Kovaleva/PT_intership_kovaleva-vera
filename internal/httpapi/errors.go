package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"urlshortener/internal/repository"
	"urlshortener/internal/service"
)

const (
	codeMalformedJSON    = "malformed_json"
	codeUnsupportedType  = "unsupported_media_type"
	codePayloadTooLarge  = "payload_too_large"
	codeMethodNotAllowed = "method_not_allowed"
	codeInvalidURL       = "invalid_url"
	codeNotFound         = "not_found"
	codeInternal         = "internal_error"
	codeUnavailable      = "service_unavailable"
)

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()

	if errors.Is(err, context.Canceled) {
		h.logger.InfoContext(ctx, "client gone")
		return
	}

	var (
		status  = http.StatusInternalServerError
		code    = codeInternal
		message = "internal error"
	)

	var (
		apiErr         *apiError
		unavailableErr *repository.UnavailableError
	)

	switch {
	case errors.As(err, &apiErr):
		status = apiErr.status
		code = apiErr.code
		message = apiErr.message

	case errors.Is(err, service.ErrInvalidURL):
		status = http.StatusUnprocessableEntity
		code = codeInvalidURL
		message = err.Error()

	case errors.Is(err, repository.ErrNotFound):
		status = http.StatusNotFound
		code = codeNotFound
		message = "short code not found"

	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &unavailableErr):
		status = http.StatusServiceUnavailable
		code = codeUnavailable
		message = "service temporarily unavailable"
	}

	if status >= http.StatusInternalServerError {
		h.logger.ErrorContext(ctx, "request failed", "error", err, "status", status)
	}

	sendJSONError(w, status, code, message)
}

func sendJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: errorBody{Code: code, Message: message}})
}

type apiError struct {
	status  int
	code    string
	message string
}

func requireJSON(r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType != "application/json" {
		return &apiError{http.StatusUnsupportedMediaType, codeUnsupportedType, "content-type must be application/json"}
	}
	return nil
}

func (e *apiError) Error() string { return e.message }

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

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		status  = http.StatusInternalServerError
		code    = codeInternal
		message = "internal error"
	)

	var unavailableErr *repository.UnavailableError

	switch {
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{Code: code, Message: message},
	})
}

func (h *Handler) sendJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{Code: code, Message: message},
	})
}

type apiError struct {
	status  int
	code    string
	message string
}

func requireJSON(r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return nil // Разрешаем curl без заголовков для удобства
	}
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType != "application/json" {
		// Возвращаем кастомную ошибку, которую h.respondError превратит в 415
		return &apiError{http.StatusUnsupportedMediaType, codeUnsupportedType, "content-type must be application/json"}
	}
	return nil
}

func (e *apiError) Error() string { return e.message }

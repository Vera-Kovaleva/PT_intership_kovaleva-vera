package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"urlshortener/internal/config"
	"urlshortener/internal/service"
)

type Shortener interface {
	Shorten(context.Context, string) (string, error)
	Resolve(context.Context, string) (string, error)
	Ping(context.Context) error
}

var _ Shortener = (*service.Service)(nil)

type Handler struct {
	srv     Shortener
	log     *slog.Logger
	baseURL string
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", h.Shorten)
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /{code}", h.Redirect)
	mux.HandleFunc("/", h.fallback)
	return mux
}

func NewHandler(srv Shortener, baseURL string) *Handler {
	return &Handler{srv: srv, baseURL: baseURL}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), config.PingTimeout)
	defer cancel()

	status, httpStatus := "ok", http.StatusOK
	if err := h.srv.Ping(ctx); err != nil {
		status, httpStatus = "degraded", http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	target, err := h.srv.Resolve(r.Context(), code)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	w.Header().Set("Location", target)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusFound)
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	if err := requireJSON(r); err != nil {
		h.respondError(w, r, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, config.MaxBodyBytes)
	var req shortenRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field == "url" {
			h.sendJSONError(w, http.StatusUnprocessableEntity, codeInvalidURL, "url must be a string")
			return
		}

		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			h.sendJSONError(w, http.StatusRequestEntityTooLarge, codePayloadTooLarge, "request body too large")
			return
		}
		h.sendJSONError(w, http.StatusBadRequest, codeMalformedJSON, "invalid json format")
		return
	}

	if req.URL == nil {
		h.sendJSONError(w, http.StatusUnprocessableEntity, codeInvalidURL, "url field is required")
		return
	}

	code, err := h.srv.Shorten(r.Context(), *req.URL)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(shortenResponse{
		ShortURL: h.baseURL + "/" + code,
	})

}

type shortenRequest struct {
	URL *string `json:"url"`
}

type shortenResponse struct {
	ShortURL string `json:"short_url"`
}

var knownPaths = map[string]string{
	"/shorten": http.MethodPost,
	"/health":  http.MethodGet,
}

func (h *Handler) fallback(w http.ResponseWriter, r *http.Request) {
	if allowed, ok := knownPaths[r.URL.Path]; ok {
		w.Header().Set("Allow", allowed)
		h.sendJSONError(w, http.StatusMethodNotAllowed, codeMethodNotAllowed, "method not allowed for this path")
		return
	}
	h.sendJSONError(w, http.StatusNotFound, codeNotFound, "not found")
}

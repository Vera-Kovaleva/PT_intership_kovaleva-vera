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

type Shortner interface {
	Shorten(context.Context, string) (string, error)
	Resolve(context.Context, string) (string, error)
	Ping(context.Context) error
}

var _ Shortner = (*service.Service)(nil)

type Handler struct {
	srv     Shortner
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

func NewHandler(srv Shortner, baseURL string) *Handler {
	return &Handler{srv: srv, baseURL: baseURL}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), config.PingTimeout)
	defer cancel()

	if err := h.srv.Ping(ctx); err != nil {
		h.respondError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
		h.sendJSONError(w, 405, codeMethodNotAllowed, "method not allowed for this path")
		return
	}
	h.sendJSONError(w, 404, codeNotFound, "not found")
}

package task

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/devrashed01/go-project/internal/httpx"
	"github.com/devrashed01/go-project/internal/middleware"
	"github.com/devrashed01/go-project/internal/validator"
)

// Handler exposes the task service over HTTP. Its only job is translating
// between HTTP and the service: parse the request, call the service, write
// the response.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler creates a Handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// RegisterRoutes adds the task routes to mux. Method and path patterns such
// as "GET /tasks/{id}" are supported by net/http since Go 1.22.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tasks", h.create)
	mux.HandleFunc("GET /api/v1/tasks", h.list)
	mux.HandleFunc("GET /api/v1/tasks/{id}", h.get)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", h.delete)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	t, err := h.svc.Create(r.Context(), in)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/tasks/"+strconv.FormatInt(t.ID, 10))
	httpx.JSON(w, http.StatusCreated, httpx.Envelope{Data: t})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	t, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.Envelope{Data: t})
}

type listMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f ListFilter
	var v validator.Validator

	if s := q.Get("status"); s != "" {
		st := Status(s)
		f.Status = &st
	}
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		v.Check(err == nil, "limit", "must be an integer")
		f.Limit = n
	}
	if s := q.Get("offset"); s != "" {
		n, err := strconv.Atoi(s)
		v.Check(err == nil, "offset", "must be an integer")
		f.Offset = n
	}
	if err := v.Err(); err != nil {
		h.handleError(w, r, err)
		return
	}

	tasks, applied, err := h.svc.List(r.Context(), f)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.Envelope{
		Data: tasks,
		Meta: listMeta{Limit: applied.Limit, Offset: applied.Offset, Count: len(tasks)},
	})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var in UpdateInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	t, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.Envelope{Data: t})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleError maps domain errors to HTTP responses. Unexpected errors are
// logged in full but the client only sees a generic message, so internal
// details (SQL, stack traces) never leak.
func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *validator.Error
	switch {
	case errors.As(err, &verr):
		httpx.Error(w, http.StatusUnprocessableEntity, "validation failed", verr.Fields)
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "task not found", nil)
	default:
		h.log.ErrorContext(r.Context(), "request failed",
			"request_id", middleware.RequestIDFrom(r.Context()),
			"err", err,
		)
		httpx.Error(w, http.StatusInternalServerError, "internal server error", nil)
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		httpx.Error(w, http.StatusBadRequest, "id must be a positive integer", nil)
		return 0, false
	}
	return id, true
}

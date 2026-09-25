// Package server builds the HTTP router and server.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/devrashed01/go-project/internal/config"
	"github.com/devrashed01/go-project/internal/httpx"
	"github.com/devrashed01/go-project/internal/middleware"
	"github.com/devrashed01/go-project/internal/task"
)

// Pinger is anything that can report whether it is reachable, such as a
// database pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Routes registers every route and wraps them in the shared middleware.
func Routes(log *slog.Logger, db Pinger, tasks *task.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(db))
	tasks.RegisterRoutes(mux)

	return middleware.Chain(mux,
		middleware.RequestID,
		middleware.Logger(log),
		middleware.Recoverer(log),
	)
}

// New creates an http.Server with production-safe timeouts. The defaults in
// net/http have no timeouts at all, which leaves the server open to slow
// clients holding connections forever.
func New(cfg config.HTTP, handler http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
}

// healthz is a liveness probe: the process is up and serving HTTP.
func healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz is a readiness probe: the app can reach its dependencies.
func readyz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "database unavailable", nil)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

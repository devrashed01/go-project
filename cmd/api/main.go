// Command api runs the Task Manager REST API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/devrashed01/go-project/internal/config"
	"github.com/devrashed01/go-project/internal/database"
	"github.com/devrashed01/go-project/internal/server"
	"github.com/devrashed01/go-project/internal/task"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run is the composition root: it builds every dependency and wires them
// together. Keeping main tiny and returning errors from run makes startup
// easy to read and test.
func run() error {
	// ctx is cancelled on Ctrl+C or SIGTERM (what Docker/Kubernetes send).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := newLogger(cfg)
	slog.SetDefault(log)

	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	taskHandler := task.NewHandler(task.NewService(task.NewPostgresRepository(pool)), log)
	srv := server.New(cfg.HTTP, server.Routes(log, pool, taskHandler), log)

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server starting", "addr", cfg.HTTP.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	// Graceful shutdown: stop accepting new connections and let in-flight
	// requests finish, up to the timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("server stopped")
	return nil
}

// newLogger uses readable text logs in development and JSON logs elsewhere,
// since JSON is what log aggregators (Datadog, Loki, CloudWatch) ingest.
func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.IsDevelopment() {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// Package config loads application configuration from environment variables.
//
// Following the twelve-factor app methodology, all configuration comes from
// the environment so the same binary can run in any environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Config is the root configuration for the application.
type Config struct {
	Env      string // "development" or "production"
	LogLevel slog.Level
	HTTP     HTTP
	DB       DB
}

// HTTP holds HTTP server settings.
type HTTP struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// DB holds database settings.
type DB struct {
	URL      string
	MaxConns int32
}

// Load reads configuration from the environment, applying defaults where a
// variable is not set. It reports every invalid variable at once rather than
// failing on the first one.
func Load() (Config, error) {
	l := &loader{}

	cfg := Config{
		Env: l.str("APP_ENV", "development"),
		HTTP: HTTP{
			Addr:            l.str("HTTP_ADDR", ":8080"),
			ReadTimeout:     l.duration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:    l.duration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:     l.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: l.duration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		DB: DB{
			URL:      l.required("DATABASE_URL"),
			MaxConns: l.int32("DB_MAX_CONNS", 10),
		},
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(l.str("LOG_LEVEL", "info"))); err != nil {
		l.errs = append(l.errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	if err := errors.Join(l.errs...); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// IsDevelopment reports whether the app runs in development mode.
func (c Config) IsDevelopment() bool { return c.Env == "development" }

// loader collects parse errors so they can be reported together.
type loader struct{ errs []error }

func (l *loader) str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func (l *loader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		l.errs = append(l.errs, fmt.Errorf("%s is required", key))
	}
	return v
}

// int32 parses a positive int32. Parsing with bitSize 32 makes strconv reject
// values that would overflow, instead of silently wrapping around.
func (l *loader) int32(key string, def int32) int32 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || n < 1 {
		l.errs = append(l.errs, fmt.Errorf("%s: must be a positive integer, got %q", key, v))
		return def
	}
	return int32(n)
}

func (l *loader) duration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s: must be a duration like 10s, got %q", key, v))
		return def
	}
	return d
}

// Package httpx contains small helpers for reading and writing JSON over HTTP,
// so every handler produces responses with the same shape.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const maxBodyBytes = 1 << 20 // 1 MB

// Envelope wraps successful responses: {"data": ..., "meta": ...}.
type Envelope struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

// ErrorResponse wraps error responses: {"error": {"message": ..., "details": ...}}.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody describes what went wrong.
type ErrorBody struct {
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are already sent, so all we can do is log.
		slog.Error("encode json response", "err", err)
	}
}

// Error writes a JSON error response.
func Error(w http.ResponseWriter, status int, message string, details any) {
	JSON(w, status, ErrorResponse{Error: ErrorBody{Message: message, Details: details}})
}

// DecodeJSON decodes a single JSON object from the request body into dst.
// It rejects unknown fields and oversized bodies, and returns errors whose
// messages are safe to show to API clients.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxErr *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxErr):
			return fmt.Errorf("malformed JSON at position %d", syntaxErr.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("malformed JSON")
		case errors.As(err, &typeErr):
			return fmt.Errorf("invalid type for field %q", typeErr.Field)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			return fmt.Errorf("unknown field %s", strings.TrimPrefix(err.Error(), "json: unknown field "))
		case errors.Is(err, io.EOF):
			return errors.New("request body must not be empty")
		case errors.As(err, &maxErr):
			return fmt.Errorf("request body must not exceed %d bytes", maxErr.Limit)
		default:
			return fmt.Errorf("invalid request body: %w", err)
		}
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

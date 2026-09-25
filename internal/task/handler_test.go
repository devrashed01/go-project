package task

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer wires the real handler and service to an in-memory repo, so
// these tests exercise routing, JSON handling and status codes end to end.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	NewHandler(NewService(newMemRepo()), log).RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// result is a fully read HTTP response. Returning plain values (rather than
// *http.Response) means callers never have to remember to close a body.
type result struct {
	status int
	header http.Header
	body   map[string]any
}

func do(t *testing.T, method, url, body string) result {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	res := result{status: resp.StatusCode, header: resp.Header}
	if resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(&res.body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return res
}

func TestHandler_CreateAndGet(t *testing.T) {
	srv := newTestServer(t)

	res := do(t, http.MethodPost, srv.URL+"/api/v1/tasks", `{"title":"Learn Go"}`)
	if res.status != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %v", res.status, res.body)
	}
	if loc := res.header.Get("Location"); loc != "/api/v1/tasks/1" {
		t.Errorf("Location = %q", loc)
	}

	res = do(t, http.MethodGet, srv.URL+"/api/v1/tasks/1", "")
	if res.status != http.StatusOK {
		t.Fatalf("get: status = %d", res.status)
	}
	data := res.body["data"].(map[string]any)
	if data["title"] != "Learn Go" || data["status"] != "todo" {
		t.Errorf("unexpected task: %v", data)
	}
}

func TestHandler_Errors(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"malformed json", http.MethodPost, "/api/v1/tasks", `{"title":`, http.StatusBadRequest},
		{"unknown field", http.MethodPost, "/api/v1/tasks", `{"title":"x","priority":1}`, http.StatusBadRequest},
		{"empty body", http.MethodPost, "/api/v1/tasks", ``, http.StatusBadRequest},
		{"validation", http.MethodPost, "/api/v1/tasks", `{"title":""}`, http.StatusUnprocessableEntity},
		{"invalid id", http.MethodGet, "/api/v1/tasks/abc", ``, http.StatusBadRequest},
		{"not found", http.MethodGet, "/api/v1/tasks/999", ``, http.StatusNotFound},
		{"bad limit", http.MethodGet, "/api/v1/tasks?limit=abc", ``, http.StatusUnprocessableEntity},
		{"bad status filter", http.MethodGet, "/api/v1/tasks?status=nope", ``, http.StatusUnprocessableEntity},
		{"delete missing", http.MethodDelete, "/api/v1/tasks/999", ``, http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := do(t, tt.method, srv.URL+tt.path, tt.body)
			if res.status != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %v)", res.status, tt.wantStatus, res.body)
			}
			if _, ok := res.body["error"]; !ok {
				t.Errorf("error responses must have an \"error\" key, got %v", res.body)
			}
		})
	}
}

func TestHandler_UpdateListDelete(t *testing.T) {
	srv := newTestServer(t)
	do(t, http.MethodPost, srv.URL+"/api/v1/tasks", `{"title":"one"}`)
	do(t, http.MethodPost, srv.URL+"/api/v1/tasks", `{"title":"two"}`)

	res := do(t, http.MethodPatch, srv.URL+"/api/v1/tasks/1", `{"status":"done"}`)
	if res.status != http.StatusOK {
		t.Fatalf("patch: status = %d, body = %v", res.status, res.body)
	}

	res = do(t, http.MethodGet, srv.URL+"/api/v1/tasks?status=done", "")
	if n := len(res.body["data"].([]any)); n != 1 {
		t.Errorf("filter by status: got %d tasks, want 1", n)
	}

	res = do(t, http.MethodDelete, srv.URL+"/api/v1/tasks/1", "")
	if res.status != http.StatusNoContent {
		t.Fatalf("delete: status = %d", res.status)
	}

	res = do(t, http.MethodGet, srv.URL+"/api/v1/tasks", "")
	if n := len(res.body["data"].([]any)); n != 1 {
		t.Errorf("after delete: got %d tasks, want 1", n)
	}
}

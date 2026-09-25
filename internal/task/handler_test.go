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

func do(t *testing.T, method, url, body string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var out map[string]any
	if resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp, out
}

func TestHandler_CreateAndGet(t *testing.T) {
	srv := newTestServer(t)

	resp, body := do(t, http.MethodPost, srv.URL+"/api/v1/tasks", `{"title":"Learn Go"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %v", resp.StatusCode, body)
	}
	if loc := resp.Header.Get("Location"); loc != "/api/v1/tasks/1" {
		t.Errorf("Location = %q", loc)
	}

	resp, body = do(t, http.MethodGet, srv.URL+"/api/v1/tasks/1", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: status = %d", resp.StatusCode)
	}
	data := body["data"].(map[string]any)
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
			resp, body := do(t, tt.method, srv.URL+tt.path, tt.body)
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %v)", resp.StatusCode, tt.wantStatus, body)
			}
			if _, ok := body["error"]; !ok {
				t.Errorf("error responses must have an \"error\" key, got %v", body)
			}
		})
	}
}

func TestHandler_UpdateListDelete(t *testing.T) {
	srv := newTestServer(t)
	do(t, http.MethodPost, srv.URL+"/api/v1/tasks", `{"title":"one"}`)
	do(t, http.MethodPost, srv.URL+"/api/v1/tasks", `{"title":"two"}`)

	resp, body := do(t, http.MethodPatch, srv.URL+"/api/v1/tasks/1", `{"status":"done"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch: status = %d, body = %v", resp.StatusCode, body)
	}

	_, body = do(t, http.MethodGet, srv.URL+"/api/v1/tasks?status=done", "")
	if n := len(body["data"].([]any)); n != 1 {
		t.Errorf("filter by status: got %d tasks, want 1", n)
	}

	resp, _ = do(t, http.MethodDelete, srv.URL+"/api/v1/tasks/1", "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: status = %d", resp.StatusCode)
	}

	_, body = do(t, http.MethodGet, srv.URL+"/api/v1/tasks", "")
	if n := len(body["data"].([]any)); n != 1 {
		t.Errorf("after delete: got %d tasks, want 1", n)
	}
}

//go:build integration

// Integration tests run against a real PostgreSQL database. They are behind a
// build tag so `go test ./...` stays fast and needs no database; run them with
// `make test-integration`.
package task

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/devrashed01/go-project/migrations"
)

func newTestRepo(t *testing.T) *PostgresRepository {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.UpContext(ctx, db, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE tasks RESTART IDENTITY`); err != nil {
		t.Fatal(err)
	}
	return NewPostgresRepository(pool)
}

func TestPostgresRepository_CRUD(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	due := time.Date(2030, 1, 2, 15, 0, 0, 0, time.UTC)

	created, err := repo.Create(ctx, CreateInput{Title: "Integration", Status: StatusTodo, DueDate: &due})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 || created.CreatedAt.IsZero() {
		t.Fatalf("expected DB-generated fields, got %+v", created)
	}
	if created.DueDate == nil || !created.DueDate.Equal(due) {
		t.Errorf("due_date = %v, want %v", created.DueDate, due)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil || got.Title != "Integration" {
		t.Fatalf("get: %+v, %v", got, err)
	}

	updated, err := repo.Update(ctx, created.ID, UpdateInput{Status: ptr(StatusInProgress)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Status != StatusInProgress || updated.Title != "Integration" {
		t.Errorf("update changed wrong fields: %+v", updated)
	}

	list, err := repo.List(ctx, ListFilter{Status: ptr(StatusInProgress), Limit: 10})
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %d tasks, err %v", len(list), err)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get after delete: want ErrNotFound, got %v", err)
	}
	if err := repo.Delete(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing: want ErrNotFound, got %v", err)
	}
}

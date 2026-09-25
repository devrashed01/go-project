package task

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository stores tasks in PostgreSQL.
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a PostgresRepository.
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Compile-time check that PostgresRepository satisfies Repository.
var _ Repository = (*PostgresRepository)(nil)

const taskColumns = `id, title, description, status, due_date, created_at, updated_at`

// Create inserts a task. All values are passed as query parameters ($1, $2…),
// never concatenated into the SQL string, which prevents SQL injection.
func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (Task, error) {
	const q = `
		INSERT INTO tasks (title, description, status, due_date)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + taskColumns

	row := r.db.QueryRow(ctx, q, in.Title, in.Description, string(in.Status), in.DueDate)
	return scanTask(row)
}

// Get returns one task by ID.
func (r *PostgresRepository) Get(ctx context.Context, id int64) (Task, error) {
	const q = `SELECT ` + taskColumns + ` FROM tasks WHERE id = $1`

	t, err := scanTask(r.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

// List returns tasks, newest first.
func (r *PostgresRepository) List(ctx context.Context, f ListFilter) ([]Task, error) {
	const q = `
		SELECT ` + taskColumns + `
		FROM tasks
		WHERE ($1::text IS NULL OR status = $1)
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, statusArg(f.Status), f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	tasks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Task, error) {
		return scanTask(row)
	})
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []Task{} // encode as [] rather than null
	}
	return tasks, nil
}

// Update changes only the provided fields. COALESCE keeps the current value
// when a parameter is NULL (i.e. the field was not sent).
func (r *PostgresRepository) Update(ctx context.Context, id int64, in UpdateInput) (Task, error) {
	const q = `
		UPDATE tasks SET
			title       = COALESCE($2, title),
			description = COALESCE($3, description),
			status      = COALESCE($4, status),
			due_date    = COALESCE($5, due_date),
			updated_at  = now()
		WHERE id = $1
		RETURNING ` + taskColumns

	row := r.db.QueryRow(ctx, q, id, in.Title, in.Description, statusArg(in.Status), in.DueDate)
	t, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return t, err
}

// Delete removes a task by ID.
func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	var status string
	if err := row.Scan(&t.ID, &t.Title, &t.Description, &status, &t.DueDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return Task{}, err
	}
	t.Status = Status(status)

	// pgx returns timestamps in the server's local zone; APIs should speak UTC.
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	if t.DueDate != nil {
		utc := t.DueDate.UTC()
		t.DueDate = &utc
	}
	return t, nil
}

// statusArg converts an optional status to a value pgx sends as text or NULL.
func statusArg(s *Status) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

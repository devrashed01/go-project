// Package task implements the task feature: domain model, business logic,
// persistence and HTTP handlers. Code is grouped by feature (not by layer)
// so everything about tasks lives in one place.
package task

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devrashed01/go-project/internal/validator"
)

// ErrNotFound is returned when a task does not exist.
var ErrNotFound = errors.New("task not found")

const (
	maxTitleLen       = 200
	maxDescriptionLen = 2000
)

// Status is the workflow state of a task.
type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone:
		return true
	}
	return false
}

// Task is the core domain model.
type Task struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	DueDate     *time.Time `json:"due_date"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateInput is the data needed to create a task.
type CreateInput struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	DueDate     *time.Time `json:"due_date"`
}

func (in *CreateInput) normalize() {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if in.Status == "" {
		in.Status = StatusTodo
	}
}

func (in CreateInput) validate() error {
	var v validator.Validator
	checkTitle(&v, in.Title)
	checkDescription(&v, in.Description)
	checkStatus(&v, in.Status)
	return v.Err()
}

// UpdateInput holds a partial update. Nil fields are left unchanged, which
// gives PATCH semantics.
type UpdateInput struct {
	Title       *string    `json:"title"`
	Description *string    `json:"description"`
	Status      *Status    `json:"status"`
	DueDate     *time.Time `json:"due_date"`
}

func (in *UpdateInput) normalize() {
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		in.Title = &t
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		in.Description = &d
	}
}

func (in UpdateInput) validate() error {
	var v validator.Validator
	v.Check(in.Title != nil || in.Description != nil || in.Status != nil || in.DueDate != nil,
		"body", "at least one field must be provided")
	if in.Title != nil {
		checkTitle(&v, *in.Title)
	}
	if in.Description != nil {
		checkDescription(&v, *in.Description)
	}
	if in.Status != nil {
		checkStatus(&v, *in.Status)
	}
	return v.Err()
}

// ListFilter controls which tasks are listed and how they are paginated.
type ListFilter struct {
	Status *Status
	Limit  int
	Offset int
}

const (
	defaultLimit = 20
	maxLimit     = 100
)

func (f *ListFilter) normalize() {
	if f.Limit == 0 {
		f.Limit = defaultLimit
	}
}

func (f ListFilter) validate() error {
	var v validator.Validator
	v.Check(f.Limit >= 1 && f.Limit <= maxLimit, "limit", "must be between 1 and 100")
	v.Check(f.Offset >= 0, "offset", "must not be negative")
	if f.Status != nil {
		checkStatus(&v, *f.Status)
	}
	return v.Err()
}

func checkTitle(v *validator.Validator, title string) {
	v.Check(title != "", "title", "must not be empty")
	v.Check(utf8.RuneCountInString(title) <= maxTitleLen, "title", "must be at most 200 characters")
}

func checkDescription(v *validator.Validator, desc string) {
	v.Check(utf8.RuneCountInString(desc) <= maxDescriptionLen, "description", "must be at most 2000 characters")
}

func checkStatus(v *validator.Validator, s Status) {
	v.Check(s.Valid(), "status", "must be one of: todo, in_progress, done")
}

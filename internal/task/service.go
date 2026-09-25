package task

import (
	"context"
	"fmt"
)

// Repository is the storage the service needs. It is defined here, where it
// is consumed, rather than next to the implementation. This keeps the service
// independent of Postgres and lets tests supply an in-memory fake.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (Task, error)
	Get(ctx context.Context, id int64) (Task, error)
	List(ctx context.Context, f ListFilter) ([]Task, error)
	Update(ctx context.Context, id int64, in UpdateInput) (Task, error)
	Delete(ctx context.Context, id int64) error
}

// Service holds the business logic for tasks. Handlers call the service;
// the service calls the repository. Business rules belong here, not in HTTP
// handlers or SQL.
type Service struct {
	repo Repository
}

// NewService creates a Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates the input and stores a new task.
func (s *Service) Create(ctx context.Context, in CreateInput) (Task, error) {
	in.normalize()
	if err := in.validate(); err != nil {
		return Task{}, err
	}
	t, err := s.repo.Create(ctx, in)
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

// Get returns a single task, or ErrNotFound.
func (s *Service) Get(ctx context.Context, id int64) (Task, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return Task{}, fmt.Errorf("get task %d: %w", id, err)
	}
	return t, nil
}

// List returns tasks matching the filter. It returns the normalized filter so
// callers can report the pagination values that were actually applied.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Task, ListFilter, error) {
	f.normalize()
	if err := f.validate(); err != nil {
		return nil, f, err
	}
	tasks, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, f, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, f, nil
}

// Update applies a partial update to a task.
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (Task, error) {
	in.normalize()
	if err := in.validate(); err != nil {
		return Task{}, err
	}
	t, err := s.repo.Update(ctx, id, in)
	if err != nil {
		return Task{}, fmt.Errorf("update task %d: %w", id, err)
	}
	return t, nil
}

// Delete removes a task, or returns ErrNotFound.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	return nil
}

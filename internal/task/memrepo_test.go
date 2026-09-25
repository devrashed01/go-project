package task

import (
	"context"
	"slices"
	"sync"
	"time"
)

// memRepo is an in-memory Repository used by unit tests. Because the service
// depends on the Repository interface, tests can run without a database.
type memRepo struct {
	mu     sync.Mutex
	tasks  map[int64]Task
	nextID int64
}

func newMemRepo() *memRepo {
	return &memRepo{tasks: make(map[int64]Task)}
}

func (m *memRepo) Create(_ context.Context, in CreateInput) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	now := time.Now().UTC()
	t := Task{
		ID: m.nextID, Title: in.Title, Description: in.Description,
		Status: in.Status, DueDate: in.DueDate, CreatedAt: now, UpdatedAt: now,
	}
	m.tasks[t.ID] = t
	return t, nil
}

func (m *memRepo) Get(_ context.Context, id int64) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return t, nil
}

func (m *memRepo) List(_ context.Context, f ListFilter) ([]Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Task{}
	for _, t := range m.tasks {
		if f.Status == nil || t.Status == *f.Status {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b Task) int { return int(b.ID - a.ID) })
	if f.Offset >= len(out) {
		return []Task{}, nil
	}
	out = out[f.Offset:]
	if len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func (m *memRepo) Update(_ context.Context, id int64, in UpdateInput) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	if in.Title != nil {
		t.Title = *in.Title
	}
	if in.Description != nil {
		t.Description = *in.Description
	}
	if in.Status != nil {
		t.Status = *in.Status
	}
	if in.DueDate != nil {
		t.DueDate = in.DueDate
	}
	t.UpdatedAt = time.Now().UTC()
	m.tasks[id] = t
	return t, nil
}

func (m *memRepo) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(m.tasks, id)
	return nil
}

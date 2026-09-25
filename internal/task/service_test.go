package task

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devrashed01/go-project/internal/validator"
)

func ptr[T any](v T) *T { return &v }

// Table-driven tests are the idiomatic Go way to cover many cases compactly.
func TestServiceCreate_Validation(t *testing.T) {
	tests := []struct {
		name      string
		in        CreateInput
		wantField string // "" means no validation error expected
	}{
		{name: "valid", in: CreateInput{Title: "Write tests"}},
		{name: "title is trimmed then required", in: CreateInput{Title: "   "}, wantField: "title"},
		{name: "title too long", in: CreateInput{Title: strings.Repeat("a", 201)}, wantField: "title"},
		{name: "description too long", in: CreateInput{Title: "ok", Description: strings.Repeat("a", 2001)}, wantField: "description"},
		{name: "unknown status", in: CreateInput{Title: "ok", Status: "archived"}, wantField: "status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(newMemRepo())
			_, err := svc.Create(context.Background(), tt.in)

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var verr *validator.Error
			if !errors.As(err, &verr) {
				t.Fatalf("want validation error, got %v", err)
			}
			if _, ok := verr.Fields[tt.wantField]; !ok {
				t.Errorf("want error on field %q, got %v", tt.wantField, verr.Fields)
			}
		})
	}
}

func TestServiceCreate_DefaultsStatusToTodo(t *testing.T) {
	svc := NewService(newMemRepo())

	got, err := svc.Create(context.Background(), CreateInput{Title: "  Buy milk  "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusTodo {
		t.Errorf("status = %q, want %q", got.Status, StatusTodo)
	}
	if got.Title != "Buy milk" {
		t.Errorf("title = %q, want trimmed %q", got.Title, "Buy milk")
	}
}

func TestServiceUpdate(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemRepo())
	created, _ := svc.Create(ctx, CreateInput{Title: "Original", Description: "keep me"})

	got, err := svc.Update(ctx, created.ID, UpdateInput{Status: ptr(StatusDone)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusDone {
		t.Errorf("status = %q, want %q", got.Status, StatusDone)
	}
	if got.Title != "Original" || got.Description != "keep me" {
		t.Errorf("fields not sent should be unchanged, got %+v", got)
	}
}

func TestServiceUpdate_EmptyBodyIsInvalid(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.Update(context.Background(), 1, UpdateInput{})

	var verr *validator.Error
	if !errors.As(err, &verr) {
		t.Fatalf("want validation error, got %v", err)
	}
}

func TestServiceGet_NotFound(t *testing.T) {
	svc := NewService(newMemRepo())
	_, err := svc.Get(context.Background(), 42)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestServiceList_Pagination(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemRepo())

	_, applied, err := svc.List(ctx, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Limit != defaultLimit {
		t.Errorf("default limit = %d, want %d", applied.Limit, defaultLimit)
	}

	_, _, err = svc.List(ctx, ListFilter{Limit: maxLimit + 1})
	var verr *validator.Error
	if !errors.As(err, &verr) {
		t.Fatalf("limit above max: want validation error, got %v", err)
	}
}

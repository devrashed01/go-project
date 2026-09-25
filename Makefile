# Load variables from .env (if present) and export them to every command.
-include .env
export

BINARY := bin/api

.DEFAULT_GOAL := help
.PHONY: help run build test test-integration cover lint fmt tidy \
        migrate-up migrate-down migrate-status db-create

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

run: ## Run the API locally
	go run ./cmd/api

build: ## Build a static binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/api

test: ## Run unit tests (no database needed)
	go test -race -count=1 ./...

test-integration: ## Run unit + integration tests (needs TEST_DATABASE_URL)
	go test -race -count=1 -tags=integration ./...

cover: ## Run tests and open an HTML coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format all Go code
	gofmt -s -w .

tidy: ## Tidy and verify go.mod
	go mod tidy
	go mod verify

db-create: ## Create the dev and test databases in local Postgres
	createdb tasks || true
	createdb tasks_test || true

migrate-up: ## Apply all pending migrations
	go run ./cmd/migrate up

migrate-down: ## Roll back the most recent migration
	go run ./cmd/migrate down

migrate-status: ## Show migration status
	go run ./cmd/migrate status

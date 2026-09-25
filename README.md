# Task Manager API

A small REST API in Go, built the way production services are built. The
domain is intentionally simple (tasks) so the focus stays on **structure and
best practices**.

## Tech stack

| Concern        | Choice                                   | Why |
|----------------|------------------------------------------|-----|
| HTTP routing   | `net/http` standard library (Go 1.22+)   | Method + path patterns are built in; no framework needed |
| Database       | PostgreSQL via [`pgx`](https://github.com/jackc/pgx) | Fastest, most complete Postgres driver for Go |
| Migrations     | [`goose`](https://github.com/pressly/goose), embedded in the binary | Versioned, reversible schema changes |
| Logging        | `log/slog` (standard library)            | Structured logs; JSON in production |
| Config         | Environment variables                    | [12-factor](https://12factor.net/config) |
| Testing        | `testing` + `httptest`                   | No test framework needed |

## Project layout

```
.
├── cmd/
│   ├── api/            # main entrypoint: wires everything together
│   └── migrate/        # runs DB migrations
├── internal/           # private application code (can't be imported by other modules)
│   ├── config/         # env var loading + validation
│   ├── database/       # Postgres connection pool
│   ├── httpx/          # JSON request/response helpers
│   ├── middleware/     # request ID, logging, panic recovery
│   ├── server/         # router, http.Server, health checks
│   ├── validator/      # field validation errors
│   └── task/           # the "task" feature
│       ├── task.go         # domain model + validation rules
│       ├── service.go      # business logic + Repository interface
│       ├── postgres.go     # Repository implementation (SQL)
│       └── handler.go      # HTTP handlers
├── migrations/         # SQL migrations (embedded into the binary)
├── Dockerfile / docker-compose.yml
├── Makefile
└── .github/workflows/ci.yml
```

### How a request flows

```
HTTP request
  → middleware (request ID → logging → panic recovery)
    → Handler   parse/validate HTTP input, map errors to status codes
      → Service   business rules, validation
        → Repository (interface)   ← PostgresRepository in prod, memRepo in tests
          → PostgreSQL
```

Each layer only knows about the one below it. The service depends on an
**interface**, not on Postgres, which is what makes it testable without a
database.

## Getting started

Prerequisites: Go 1.25+, PostgreSQL running locally.

```bash
cp .env.example .env     # then edit DATABASE_URL for your machine
make db-create           # creates `tasks` and `tasks_test` databases
make migrate-up          # creates the tables
make run                 # starts the API on :8080
```

Or with Docker (no local Postgres needed):

```bash
docker compose up --build
```

Run `make` with no arguments to list every command.

## API

Base path: `/api/v1`

| Method | Path          | Description              | Success |
|--------|---------------|--------------------------|---------|
| POST   | `/tasks`      | Create a task            | 201 |
| GET    | `/tasks`      | List tasks (`?status=&limit=&offset=`) | 200 |
| GET    | `/tasks/{id}` | Get one task             | 200 |
| PATCH  | `/tasks/{id}` | Partially update a task  | 200 |
| DELETE | `/tasks/{id}` | Delete a task            | 204 |
| GET    | `/healthz`    | Liveness probe           | 200 |
| GET    | `/readyz`     | Readiness probe (checks DB) | 200 / 503 |

### Examples

```bash
curl -X POST localhost:8080/api/v1/tasks \
  -d '{"title":"Learn Go","description":"Build a REST API","due_date":"2026-12-31T17:00:00Z"}'

curl "localhost:8080/api/v1/tasks?status=todo&limit=10"

curl -X PATCH localhost:8080/api/v1/tasks/1 -d '{"status":"done"}'

curl -X DELETE localhost:8080/api/v1/tasks/1
```

### Response format

Every response has the same shape, so clients can handle them uniformly.

```json
// success
{ "data": { "id": 1, "title": "Learn Go", "status": "todo", ... } }

// list
{ "data": [ ... ], "meta": { "limit": 20, "offset": 0, "count": 3 } }

// error
{ "error": { "message": "validation failed",
             "details": { "title": "must not be empty" } } }
```

| Status | Meaning |
|--------|---------|
| 400 | Malformed request (bad JSON, unknown field, bad ID) |
| 404 | Task does not exist |
| 422 | Well-formed but invalid data (validation) |
| 500 | Unexpected error. Details are logged, never sent to the client |

## Testing

```bash
make test               # unit tests: fast, no database
make test-integration   # also runs the Postgres repository tests
make cover              # HTML coverage report
```

- **Unit tests** (`service_test.go`, `handler_test.go`) use an in-memory fake
  repository (`memrepo_test.go`) and `httptest`.
- **Integration tests** (`postgres_test.go`) are behind the `integration`
  build tag and run against a real database set by `TEST_DATABASE_URL`.

## Best practices used in this project

1. **`internal/` package**: the compiler blocks other modules from importing it.
2. **Package by feature**: all task code lives in `internal/task`.
3. **Accept interfaces, return structs**: `Service` takes a `Repository`
   interface declared where it is *used*.
4. **Dependency injection by hand**: `cmd/api/main.go` is the single place
   that builds and connects everything. No globals, no DI framework.
5. **Errors wrap context**: `fmt.Errorf("get task %d: %w", id, err)`, checked
   with `errors.Is` / `errors.As`.
6. **Sentinel & typed errors mapped to HTTP** in exactly one place (`handleError`).
7. **Never leak internals**: 500s return a generic message; details go to logs
   with the request ID.
8. **Parameterized SQL only**: no string concatenation, so no SQL injection.
9. **Timeouts everywhere**: HTTP server, DB ping, readiness probe.
10. **Graceful shutdown**: in-flight requests finish on SIGTERM.
11. **Structured logging** with a request ID on every line.
12. **Request hardening**: 1 MB body limit, unknown JSON fields rejected.
13. **Database constraints** (`CHECK`) back up application validation.
14. **Config validated at startup**: the app fails fast and lists every problem.
15. **Minimal, non-root Docker image** (distroless) built in multiple stages.
16. **CI** runs vet, tests (with race detector) and lint on every PR.

## Ideas to extend (good next exercises)

- Add a `users` feature with registration and JWT authentication
- Add a `total` count to list responses
- Allow clearing `due_date` in PATCH (hint: explicit `null` vs. field absent)
- Generate typed queries with [`sqlc`](https://sqlc.dev)
- Add OpenAPI docs
- Add rate limiting and CORS middleware
- Add Prometheus metrics and OpenTelemetry tracing

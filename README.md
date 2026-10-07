# go-rest-api

A production-style RESTful API in Go: JWT authentication, per-user task management,
and a storage layer that swaps between in-memory and PostgreSQL.

Built with [Gin](https://github.com/gin-gonic/gin), `golang-jwt`, and `pgx`.

## Features

- **Auth** — register/login with bcrypt-hashed passwords, HMAC-SHA256 JWTs
- **Tasks CRUD** — per-user isolation, status filter, text search, pagination
- **Storage** — `Store` interface with two backends: thread-safe in-memory
  (default, zero setup) and PostgreSQL (via `DATABASE_URL`, auto-migrated)
- **Ops-ready** — structured `slog` logs, request IDs, per-IP rate limiting,
  CORS, graceful shutdown, `/healthz`
- **Docs & tests** — OpenAPI 3 spec in `docs/`, handler + auth unit tests,
  multi-stage Dockerfile, docker-compose for local Postgres

## Quickstart

```bash
# zero-dependency run (in-memory store)
go run ./cmd/api

# register + create a task
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo","password":"password123"}' | jq -r .token)

curl -X POST localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Ship portfolio","status":"in_progress"}'
```

With Postgres:

```bash
docker compose up --build
# DATABASE_URL is wired automatically; tables are created on startup
```

## Configuration

| Variable       | Default              | Description                              |
|----------------|----------------------|------------------------------------------|
| `PORT`         | `8080`               | HTTP listen port                         |
| `JWT_SECRET`   | `dev-only-change-me` | HMAC secret for tokens (change in prod!) |
| `TOKEN_TTL`    | `24h`                | Access token lifetime                    |
| `DATABASE_URL` | *(empty)*            | Postgres DSN; empty = in-memory store    |
| `RATE_LIMIT_RPS` | `10`               | Per-IP requests per second               |

## API

| Method | Path                      | Auth | Description              |
|--------|---------------------------|------|--------------------------|
| GET    | `/healthz`                | –    | Liveness probe           |
| POST   | `/api/v1/auth/register`   | –    | Register, returns JWT    |
| POST   | `/api/v1/auth/login`      | –    | Login, returns JWT       |
| GET    | `/api/v1/me`              | JWT  | Current user             |
| POST   | `/api/v1/tasks`           | JWT  | Create task              |
| GET    | `/api/v1/tasks`           | JWT  | List (`?status=&q=&limit=&offset=`) |
| GET    | `/api/v1/tasks/:id`       | JWT  | Get one task             |
| PUT    | `/api/v1/tasks/:id`       | JWT  | Replace task             |
| DELETE | `/api/v1/tasks/:id`       | JWT  | Delete task              |

Full schema: [`docs/openapi.yaml`](docs/openapi.yaml).

## Project layout

```
cmd/api              entrypoint: config, store wiring, graceful shutdown
internal/config      env-based configuration
internal/store       domain models + Store interface
  memory.go          thread-safe in-memory backend
  postgres.go        pgx-backed backend with auto-migration
internal/auth        JWT issue/verify
internal/httpapi     Gin router, handlers, middleware
                     (request ID, slog logging, recovery, CORS, rate limit, auth)
docs/openapi.yaml    OpenAPI 3 specification
```

## Testing

```bash
go test ./...   # handler + auth tests, all against the memory store
go vet ./...
```

## License

MIT — see [LICENSE](LICENSE).

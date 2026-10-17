# RoleScout

Open-source, self-hosted job discovery and application tracking platform.

## Services

- api: Go + Fiber REST API
- web: React + TypeScript + Vite
- worker: independent job ingestion worker
- postgres: PostgreSQL

## Development

Requirements: Docker with Compose, Go 1.27+, Node.js 22+.

```bash
cp .env.example .env   # then set POSTGRES_PASSWORD
docker compose up -d --build --wait
```

| Service | URL |
| --- | --- |
| Web | http://127.0.0.1:4173 |
| API liveness | http://127.0.0.1:8080/healthz |
| API readiness (checks PostgreSQL) | http://127.0.0.1:8080/readyz |

The API applies pending SQL migrations from `api/migrations` on startup.
Migration files are named `YYYYMMDDHHMMSS_description.sql`.

### Checks

```bash
cd api && go vet ./... && go test ./...
cd web && npm ci && npm run lint && npm test && npm run build
```

Repository tests run against PostgreSQL when `TEST_DATABASE_URL` is set and are
skipped otherwise. Each run creates and drops its own `rolescout_test_*`
database, so the URL needs a role that may create databases:

```bash
TEST_DATABASE_URL="postgres://rolescout:<password>@127.0.0.1:5432/rolescout?sslmode=disable" go test -race ./...
```

Go tests live next to the code as `*_test.go` (the Go tool skips directories
starting with `_`). Frontend tests live in `__tests__` folders.

The REST API is described in [docs/api/openapi.yaml](docs/api/openapi.yaml)
with conventions in [docs/api/README.md](docs/api/README.md).
Data model decisions are recorded in [docs/adr](docs/adr).
See [docs/PHASES.md](docs/PHASES.md) for the roadmap and current status.

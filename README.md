# RoleScout

Open-source, self-hosted job discovery and application tracking.

RoleScout imports job listings from sources you review and enable, lets you
search and save them, and tracks every application: status, attempts,
interviews, follow-ups, notes and tags. It runs on one machine with Docker
Compose, keeps your data in your own PostgreSQL database, and works in
Turkish and English.

## Quick start
Requirements: Docker with the Compose plugin.

```bash
git clone https://github.com/0furkancolak/rolescout.git && cd rolescout
cp .env.example .env          # set POSTGRES_PASSWORD
docker compose up -d --build --wait
```

Open http://127.0.0.1:4173 and create your account.

## Documentation
| Topic | |
| --- | --- |
| [Installation](docs/installation.md) | Install with Docker Compose and take the first steps |
| [Configuration](docs/configuration.md) | Every environment variable |
| [Deployment](docs/deployment.md) | HTTPS and production checklist |
| [Operations](docs/operations.md) | Upgrades, migrations, backup and restore, monitoring |
| [Job sources](docs/sources/README.md) | Adding sources and the access review they need |
| [Security](docs/security.md) | Security model, review and known limitations |
| [API](docs/api/README.md) | REST conventions and the [OpenAPI contract](docs/api/openapi.yaml) |
| [Architecture decisions](docs/adr) | Data model, authentication, ingestion |
| [Changelog](CHANGELOG.md) · [Releasing](docs/RELEASING.md) · [Roadmap](docs/PHASES.md) | |

## Architecture
| Component | Technology | Role |
| --- | --- | --- |
| `web` | React, TypeScript, Vite, TanStack Query, Tailwind; served by nginx | User interface; proxies `/api` to the API |
| `api` | Go, Fiber v3, pgx | REST API, authentication, authorization, migrations |
| `worker` | Go | Scheduled source ingestion |
| `admin` | Go CLI | Source management (`docker compose run --rm admin`) |
| `postgres` | PostgreSQL 17 | Source of truth |

## Development
Requirements: Docker with Compose, Go 1.27+, Node.js 22+.

```bash
docker compose up -d --build --wait     # full stack
cd web && npm ci && npm run dev         # UI with hot reload on :5173, proxies /api to :8080
```

Checks:

```bash
cd api && gofmt -l . && go vet ./... && go test -race ./...
cd web && npm run lint && npm test && npm run build
scripts/smoke.sh                        # end to end against the running stack
```

Database tests run when `TEST_DATABASE_URL` is set and are skipped otherwise.
Each run creates and drops its own `rolescout_test_*` database, so the role
needs permission to create databases:

```bash
TEST_DATABASE_URL="postgres://rolescout:<password>@127.0.0.1:5432/rolescout?sslmode=disable" go test -race ./...
```

Conventions:
- Go tests live next to the code as `*_test.go` (the Go tool skips
  directories starting with `_`); frontend tests live in `__tests__` folders.
- Migrations are timestamped SQL files in `api/migrations`
  (`YYYYMMDDHHMMSS_description.sql`), applied by the API at startup.
- Frontend structure and conventions: [web/README.md](web/README.md).

## License
[Apache License 2.0](LICENSE)

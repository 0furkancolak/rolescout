# Configuration

RoleScout is configured with environment variables. With Docker Compose, put
them in `.env` next to `compose.yaml`; Compose passes the relevant ones to
each service. `.env.example` lists the common ones.

## Docker Compose
| Variable | Default | Purpose |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | required | Password of the database user. Compose refuses to start without it. |
| `POSTGRES_USER` | `rolescout` | Database user. |
| `POSTGRES_DB` | `rolescout` | Database name. |
| `POSTGRES_PORT` | `5432` | Host port for PostgreSQL, bound to 127.0.0.1. |
| `API_HOST_PORT` | `8080` | Host port for the API (health, readiness, metrics), bound to 127.0.0.1. |
| `WEB_PORT` | `4173` | Host port for the web app, bound to 127.0.0.1. |
| `ROLESCOUT_VERSION` | `dev` | Version string built into the binaries and logged at startup. |
| `ROLESCOUT_DOMAIN` | required with `deploy/compose.https.yaml` | Public domain served over HTTPS by Caddy. |

## API
| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | local development URL | PostgreSQL connection URL. Compose builds it from the variables above. |
| `API_HOST` / `API_PORT` | `0.0.0.0` / `8080` | Listen address. |
| `MIGRATIONS_DIR` | `migrations` | Directory of SQL migrations applied at startup. |
| `ALLOW_REGISTRATION` | `false` | Allow sign-ups after the first account. The first account can always register. |
| `COOKIE_SECURE` | `true` (Compose: `false`) | Send the session cookie only over HTTPS. Must be `true` in production. |
| `SESSION_TTL` | `720h` | Absolute session lifetime, a Go duration of at least `1m`. |
| `TRUSTED_PROXIES` | none (Compose: private ranges) | Comma-separated IPs or CIDR ranges allowed to set the client address with `X-Real-IP`. |
| `LOGIN_RATE_LIMIT` | `10` | Failed sign-ins per IP per 15 minutes; `0` disables. |
| `REGISTER_RATE_LIMIT` | `5` | Sign-ups per IP per hour; `0` disables. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |

## Worker
| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | local development URL | PostgreSQL connection URL. |
| `WORKER_TICK` | `1m` | How often to look for sources due to sync, at least `10s`. |
| `LOG_LEVEL` | `info` | As for the API. |

Per-source settings (adapter, configuration, sync interval) are stored in the
database and managed with the `admin` CLI; see [sources](sources/README.md).

## Admin CLI
Uses `DATABASE_URL` only.

## Web development server
| Variable | Default | Purpose |
| --- | --- | --- |
| `API_PROXY_TARGET` | `http://127.0.0.1:8080` | Where `npm run dev` proxies `/api`. |

## Tests
| Variable | Purpose |
| --- | --- |
| `TEST_DATABASE_URL` | Enables database tests. Each run creates and drops its own `rolescout_test_*` database, so the role needs `CREATEDB`. |

Invalid values stop the API and worker at startup with a message naming the
variable.

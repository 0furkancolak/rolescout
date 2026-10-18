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

## Crawling and research (worker)
RoleScout fetches career pages and job board APIs itself. It identifies as
`RoleScout/<version> (+CRAWL_USER_AGENT_URL)`, follows `robots.txt` and
limits how fast it calls each host.

| Variable | Default | Purpose |
| --- | --- | --- |
| `CRAWL_CONCURRENCY` | `16` | Hosts crawled in parallel (1–256). |
| `CRAWL_HOST_INTERVAL` | `1s` | Minimum gap between requests to one company site, at least `100ms`. |
| `CRAWL_VENDOR_CONCURRENCY` | `4` | Parallel requests to one job board API host (Greenhouse, Lever…), which serves many companies; requests are at least 250 ms apart per connection. |
| `CRAWL_USER_AGENT_URL` | project URL | Link in the User-Agent so site owners can find out who is crawling. |
| `CRAWL_ALLOW_PRIVATE` | `false` | Allow crawling private, loopback and link-local addresses. Leave off in production: it is the SSRF guard. Only for tests and local fixtures. |
| `DISCOVERY_INTERVAL` | `168h` | How often the company catalog is grown from Common Crawl and seeds, at least `1h`. |
| `COMPANY_SYNC_INTERVAL` | `12h` | Refresh interval for companies with matching jobs, at least `1h`. Others refresh every 3 days, companies without jobs weekly. |
| `RESEARCH_INTERVAL` | `12h` | Default schedule of research profiles, at least `1h`. |
| `JSONLD_MAX_PAGES` | `200` | Pages read per company career site without a supported job board (1–10000). |
| `MATCH_MIN_SCORE` | `40` | Lowest score (0–100) stored as a match. |
| `CLOSED_JOB_RETENTION` | `2160h` | Closed jobs nobody saved or applied to are deleted after this long; `0` keeps them. At least `24h` otherwise. |

## Local language model (optional)
Off by default. When on, a worker loop reads job descriptions with a small model
running on your own machine and extracts salary, remote policy, extra
technologies and a short summary. Nothing is sent to a hosted service. See
[ADR 0005](adr/0005-local-llm-enrichment.md); setup steps are in
[local AI](local-ai.md) once written.

| Variable | Default | Purpose |
| --- | --- | --- |
| `LLM_ENABLED` | `false` | Turn the enricher on. Everything else works without it. |
| `LLM_BASE_URL` | `http://host.docker.internal:11434/v1` | OpenAI-compatible endpoint (Ollama, llama.cpp `llama-server`, LM Studio). Use `http://127.0.0.1:11434/v1` when the worker runs on the host. Docker on a Mac cannot use the GPU, so the model server runs on the host. |
| `LLM_MODEL` | `qwen3:4b-instruct-2507-q4_K_M` | Model name as the server knows it. Required when enabled. |
| `LLM_FALLBACK_MODEL` | `qwen3:1.7b-q4_K_M` | Smaller model used while the main one keeps timing out; set empty to disable. |
| `LLM_API_KEY` | empty | Sent as a bearer token when set (llama.cpp `--api-key`). |
| `LLM_TIMEOUT` | `60s` | Limit for one model request, at least `1s`. |
| `LLM_CONCURRENCY` | `1` | Parallel requests (1–4). Keep 1 on a laptop. |
| `LLM_BATCH` | `25` | Jobs enriched per worker tick (1–500). |
| `LLM_MAX_INPUT_CHARS` | `6000` | Description text sent per job (500–50000); longer text is cut at a paragraph. |
| `LLM_MAX_OUTPUT_TOKENS` | `400` | Reply limit per job (50–4000). |

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

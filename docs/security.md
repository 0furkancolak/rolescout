# Security Review

Last reviewed: 2026-10-18 (Phase 9 career page research). Report vulnerabilities privately to the
maintainers instead of opening a public issue.

## Authentication and sessions
- Passwords are hashed with Argon2id (64 MiB, 3 passes); sessions are random
  32-byte tokens stored only as SHA-256 hashes, in an HttpOnly, SameSite=Lax
  cookie that is Secure unless `COOKIE_SECURE=false`. See
  [ADR 0002](adr/0002-authentication.md).
- Unknown emails cost the same hashing time as wrong passwords, and both
  return the same error.
- Failed sign-ins (10 per 15 minutes) and sign-ups (5 per hour) are limited per
  client IP. Only proxies listed in `TRUSTED_PROXIES` can set the client
  address through `X-Real-IP`; a test checks that a spoofed header does not
  reset the limit. Counters are per API process; running several API replicas
  multiplies the limit.
- Cross-site request forgery: state-changing requests need a JSON body or a
  non-simple method, and the session cookie is SameSite=Lax, so other sites
  cannot send authenticated writes. The API sends no CORS headers.

## Authorization and isolation
- Every workspace route checks membership on the server and answers `404` for
  workspaces the user does not belong to. Repository queries always filter by
  workspace, and composite foreign keys stop rows from linking across
  workspaces even if application code is wrong. Covered by unit, integration
  and smoke tests.

## Input validation and SQL
- Request bodies are decoded strictly (unknown fields, trailing data and bodies
  over 1 MiB are rejected); fields are validated with explicit limits.
- All SQL uses bind parameters. The only string building is the escaped
  `LIKE` pattern (`escapeLike`) and column lists built from constants. Reviewed
  with `grep` for `Sprintf` and string concatenation in the database package.
- Source data is untrusted: descriptions are converted to plain text before
  storage, URLs must be absolute http(s), and the UI renders text only and
  opens source links with `rel="noopener noreferrer"`.

## Web
- nginx sends `Content-Security-Policy` (scripts only from the same origin, no
  framing), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff` and a
  strict referrer policy, and hides its version.

## Secrets and configuration
- `.env` and other env files are ignored by git; `.env.example` contains
  placeholders only. Compose refuses to start without `POSTGRES_PASSWORD`.
- Database and API ports are published on 127.0.0.1 only. `/metrics` is
  served on the API port and is not proxied by nginx.
- The built-in `DATABASE_URL` default is for local development and contains no
  real credential. Set `DATABASE_URL` explicitly in any other environment.
- Logs contain request IDs, route templates and status codes, but no bodies,
  cookies, IP addresses or IDs from paths.

## Containers
- The API, worker and admin images run as a non-root user on Alpine with only
  CA certificates and time zone data added. The web image is
  `nginx-unprivileged`.
- Compose runs api, worker and web with read-only root filesystems, all Linux
  capabilities dropped and `no-new-privileges`.

## Dependencies
- CI runs `govulncheck` and `npm audit --audit-level=high`.
- 2026-10-17: npm audit found 0 vulnerabilities. govulncheck found none
  affecting RoleScout; it reports GO-2026-5932 in `golang.org/x/crypto`
  (the unmaintained `openpgp` package), which RoleScout does not import. No fix
  exists; the module is needed for `argon2`.

## Outbound crawling
- The worker fetches public career pages and job board APIs. Every request
  goes through `internal/crawl`: it sends an identifying User-Agent with a
  contact URL, obeys robots.txt (a 5xx robots.txt disallows the host;
  `Crawl-delay` is honoured), limits concurrency per host and per job board
  vendor, follows at most five redirects (each checked against robots.txt),
  refuses non-http(s) schemes and caps responses at 32 MiB.
- Sources run only for adapter types with a recorded review (ADR 0004).
  Workday's undocumented endpoint ships unreviewed; SmartRecruiters is not
  supported because its API host disallows crawlers. Common Crawl is not used
  because its hosts disallow crawlers.
- Page content is untrusted: JSON-LD and HTML are parsed with size and page
  limits, descriptions are stored as plain text, and links are rendered with
  `rel="noopener noreferrer"`.
- The API's `POST /companies` accepts any http(s) career page URL from a
  signed-in user; the worker later fetches it with the limits above. On a
  multi-user installation, close registration so only trusted users can add
  pages to crawl.

## Language model and API key (optional)
An administrator can connect a hosted OpenAI-compatible service in Settings → AI.
The API key is then stored **in plain text in the `app_settings` table**, like
`POSTGRES_PASSWORD` it protects only as far as the database does: restrict
access to the database and its backups (`scripts/backup.sh` includes it). The
API never returns the key, only its last four characters; it is sent only in the
`Authorization` header to the configured base URL; only the administrator can
read or change the connection. Job text then leaves your machine for that
service (the form says so). On-demand generation is limited to 60 requests an
hour per client IP, and the worker does not read the catalog on its own for a
non-local host unless the administrator turns that on.

## Local language model (optional)
With `LLM_ENABLED=true` the worker sends job descriptions, titles and, for match
explanations, a short summary of a research profile to the model server named
by `LLM_BASE_URL`. It is meant to be a server on the same computer (Ollama or
llama.cpp on the host, reached through `host.docker.internal`) so nothing
leaves the machine; the Settings screen warns when the host is neither loopback
nor a private address.
- **Prompt injection.** Job text is untrusted input. It is sent between
  delimiters with a system prompt that says it is data, the reply is constrained
  to a closed JSON schema and then validated: technologies and salary numbers
  must appear in the text, a seniority needs a cue word, country codes must agree
  with the rules, and an explanation may not name a technology or number that is
  not in the facts it was given. Model output never reaches SQL or HTML as
  anything but data, is stored in its own table and never overwrites a value the
  rules found.
- **No secrets in prompts or logs.** The API key goes only into the
  `Authorization` header; status endpoints return the host, never the path or the
  key; logs contain counts, model names and error classes, never job text.
- **Availability.** The enricher is a separate loop behind a circuit breaker; a
  dead, slow or hostile model server cannot delay a research run or an API
  request.
- **Cost of abuse.** One request at a time by default, bounded batches, a
  response size limit and a per-request timeout.

## Administrator role
- The first account is the administrator. Only it can review and enable
  adapter types, disable companies for everyone and read crawl settings
  (`/api/v1/admin/...`, `403 forbidden` otherwise; covered by handler tests).
- Company blocks and research profiles, runs and matches stay private to
  their workspace; the shared company catalog contains only public career
  page data and records no user.

## Known limitations
- No password reset or email verification yet; operators reset access by
  creating a new account while registration is open.
- Sessions cannot be listed or revoked per device from the UI.
- Rate limits are in memory and reset when the API restarts.

## SQL injection
Every SQL statement is a string literal written out in full next to its
parameters; values only ever travel as parameters (`$1`, `$2`…). Nothing builds
SQL at run time: no `fmt.Sprintf`, no concatenation, no helper that returns SQL.
Optional filters are written as `($1::text = '' OR column = $1::text)` and the
connection plans with the actual values (`plan_cache_mode=force_custom_plan`),
so such a statement is as fast as one written for exactly those filters.
`TestSQLIsStaticLiteralsOnly` (`api/internal/infrastructure/database`) fails the
build if a statement is not a plain literal. The only exception is the
migration runner, which executes the `.sql` files shipped with the application.

# Security Review

Last reviewed: 2026-10-17 (Phase 7). Report vulnerabilities privately to the
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

## Known limitations
- No password reset or email verification yet; operators reset access by
  creating a new account while registration is open.
- Sessions cannot be listed or revoked per device from the UI.
- Rate limits are in memory and reset when the API restarts.

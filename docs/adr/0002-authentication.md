# ADR 0002 - Authentication and Workspace Authorization

Status: Accepted
Date: 2026-10-17

## Context

RoleScout is self-hosted and may serve several users. Phase 4 adds
authentication and enforces that users only reach workspaces they belong to.
Operators should not need an external identity provider to get started.

## Decisions

### Email and password with server-side sessions
- Passwords are hashed with **Argon2id** (`golang.org/x/crypto/argon2`; 64 MiB,
  3 iterations, 2 lanes, 16-byte salt, 32-byte key) and stored in PHC string
  format, so parameters can be raised later and old hashes still verify.
- Credentials live in `user_credentials`, separate from `users`, so external
  identities (OIDC) can be added later without changing the user model.
- Passwords must be 12-128 characters. The upper bound limits hashing cost.
- Login always runs a hash comparison, even for unknown emails, so response
  time does not reveal which emails are registered. Login failures return one
  generic `invalid_credentials` error.

### Sessions
- A session token is 32 random bytes, sent to the browser base64url-encoded
  in the `rolescout_session` cookie. Only its SHA-256 hash is stored in
  `sessions`, so a database leak does not expose usable tokens.
- Sessions have an absolute lifetime (`SESSION_TTL`, default 720h). Logout
  deletes the session row, so revocation is immediate.
- Cookie attributes: `HttpOnly`, `SameSite=Lax`, `Path=/`, and `Secure` unless
  `COOKIE_SECURE=false`. The Compose file sets `COOKIE_SECURE=false` for local
  HTTP; deployments behind TLS must leave it enabled.
- CSRF: `SameSite=Lax` keeps browsers from sending the cookie on cross-site
  `POST`/`PUT`/`PATCH`/`DELETE`, and the API accepts only JSON bodies. Sibling
  subdomains are same-site, so the web app should not share a registrable
  domain with untrusted sites. A token-based CSRF defence can be added if that
  becomes necessary.

### Registration
- When no user exists, registration is always allowed, so a fresh install can
  create its first account. Afterwards it requires `ALLOW_REGISTRATION=true`.
  The check and insert run under an advisory lock, so two concurrent "first"
  registrations cannot both bypass the setting.
- Registering creates a personal workspace owned by the new user and signs the
  user in.

### Authorization
- Every `/api/v1` route except `auth/register` and `auth/login` requires a
  valid session (`401 unauthenticated` otherwise).
- Workspace routes require membership. Non-members get `404`, never `403`, so
  workspace and resource IDs cannot be probed.
- Roles: any member may read and change workspace data. Managing members is
  owner-only and returns `403 forbidden` to members, who already know the
  workspace exists.
- `created_by`, `saved_by` and `author_id` are filled with the authenticated
  user.

### Deferred
- Login rate limiting and lockout are part of Phase 7 (rate limiting).
- Password reset needs outbound email and is not implemented; an operator can
  reset a password with a future admin command.

## Consequences
- No external service is required to run RoleScout.
- The API is stateful per session; every authenticated request reads one
  session row (primary-key lookup).

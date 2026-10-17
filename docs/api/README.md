# RoleScout API Conventions

The contract is [openapi.yaml](openapi.yaml) (OpenAPI 3.1). Keep it in sync with
`api/internal/transport/http/routes.go` when routes change.

## Basics
- Base path: `/api/v1`. Health checks stay at `/healthz` and `/readyz`.
- JSON only, `snake_case` field names. Request bodies are decoded strictly:
  unknown fields, trailing data and bodies over 1 MiB are rejected.
- Timestamps are RFC 3339. Requests may use any offset; responses are UTC.
- IDs are UUIDs. A malformed ID in the path returns `404`, because it cannot
  name an existing resource.

## Responses
- Single resource: `{"data": {...}}`
- Collection: `{"data": [...]}`
- Paginated collection: `{"data": [...], "next_cursor": "..." | null}`
- `PUT` and `DELETE` on relationships (saved jobs, application tags) return
  `204 No Content`. `PUT` is idempotent.

## Errors
Every error uses one shape:

```json
{"error": {"code": "invalid_input", "message": "...", "fields": [{"field": "limit", "message": "..."}]}}
```

| Status | Code | Meaning |
| --- | --- | --- |
| 400 | `bad_request` | Missing or malformed JSON body |
| 401 | `unauthenticated` | No valid session cookie |
| 401 | `invalid_credentials` | Wrong email or password |
| 403 | `forbidden` | Member attempted an owner-only action |
| 403 | `registration_closed` | Sign-ups are disabled |
| 404 | `not_found` | Not found in this workspace, or malformed ID |
| 409 | `conflict` | Duplicate (application for a job, tag name) |
| 409 | `invalid_transition` | Status change not allowed by the workflow |
| 422 | `invalid_input` | Field validation failed; see `fields` |
| 500 | `internal` | Unexpected error; details are logged, never returned |

## Pagination, filtering and sorting
- Job search uses keyset pagination: `limit` (1-100, default 20) and an opaque
  `cursor` taken from the previous page's `next_cursor`. Results are sorted by
  `last_seen_at` descending, then `id`; other sort orders are not offered yet.
- Filters are query parameters. Enumerations reject unknown values with `422`.
  `technology` takes a comma-separated list and matches jobs that have all of
  them.
- Workspace collections (applications, saved jobs, tags, notes) are not
  paginated yet because they are bounded by one workspace's activity.

## Authentication
- `POST /api/v1/auth/register` and `/auth/login` set the `rolescout_session`
  cookie (HttpOnly, SameSite=Lax, Secure unless `COOKIE_SECURE=false`).
  `/auth/logout` revokes the session. Details: docs/adr/0002-authentication.md.
- Every other `/api/v1` route returns `401 unauthenticated` without a valid
  session, including unknown routes, so route names are not revealed.
- Login failures always return `401 invalid_credentials`, whether the email
  exists or not.
- Registration is open while the installation has no users; afterwards it
  needs `ALLOW_REGISTRATION=true`, otherwise `403 registration_closed`.

## Workspaces and authorization
Workspace data lives under `/api/v1/workspaces/{workspaceID}` and requires
membership. Resources of workspaces the user does not belong to always return
`404`, never `403`, so IDs cannot be probed. Members can read and change all
workspace data; only owners can add members (`403 forbidden` for members).
`created_by`, `saved_by` and `author_id` record the signed-in user.

## Business rules
Application status transitions and the effect of recording an attempt are
defined in `api/internal/domain/rules.go` and documented on the
`PATCH /applications/{id}` and `POST .../attempts` operations.

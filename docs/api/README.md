# RoleScout API Conventions

The contract is [openapi.yaml](openapi.yaml) (OpenAPI 3.1). Keep it in sync with
`api/internal/transport/http/routes.go` when routes change.

## Basics
- Base path: `/api/v1`. Health checks stay at `/healthz` and `/readyz`.
- JSON only, `snake_case` field names. Requests with a body must send
  `Content-Type: application/json`. Request bodies are decoded strictly:
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
| 415 | `unsupported_media_type` | Body sent without `Content-Type: application/json` |
| 422 | `invalid_input` | Field validation failed; see `fields` |
| 429 | `rate_limited` | Too many sign-in or sign-up attempts from one IP |
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
- Per IP address, 10 failed sign-ins per 15 minutes (`LOGIN_RATE_LIMIT`) and 5
  sign-ups per hour (`REGISTER_RATE_LIMIT`) are allowed; more return
  `429 rate_limited`. Counters live in the API process memory. Behind a proxy,
  set `TRUSTED_PROXIES` so the client address is read from `X-Real-IP`.
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

## Career page research
- Research profiles (`/workspaces/{id}/research-profiles`) say what a
  workspace looks for. Every workspace starts with a "Full Stack Developer"
  profile. `PATCH` changes only the fields it sends.
- `POST .../research-profiles/{id}/runs` queues a run (`202`); a profile has at
  most one queued or running run (`409`). Profiles also run on their own every
  `schedule_hours`.
- A run moves through `discovery`, `detection`, `collection`, `scoring` and
  `result`. `GET .../research-runs/{id}` returns each stage's state and
  counts; poll it (the web app polls every 2 seconds) and read new log lines
  with `GET .../research-runs/{id}/events?after=<last id>`.
- `GET .../activity?days=14` returns the dashboard's zero-filled daily series (`days`, `new_matches`, `jobs_collected`, `applications_created`) and `upcoming_interviews`.
- `GET /admin/llm` and `POST /admin/llm/check` (administrator) report the optional
  local model: reachable, model present, degraded, queue counters and whether the
  host is local. A check also sends one tiny request, at most one per 5 seconds.
  When the feature is off both answer `{enabled: false}` without a network call.
  Jobs and matches carry `ai` (what the model added: summary, remote policy,
  salary, the fields it filled, extra technologies) and `match.explanation`; both
  are `null` unless the model has read the job.
- `POST .../research-runs/{id}/cancel` cancels a queued run at once and stops
  a running one at its next progress report.
- The company catalog is shared by all workspaces. `GET
  /workspaces/{id}/companies` marks companies the workspace blocked;
  `PUT`/`DELETE .../company-blocks/{companyID}` hide or show their jobs in that
  workspace only. `POST /companies` adds a career page.
- `/admin/...` is for the installation administrator (the first account) and
  answers `403 forbidden` to everyone else.

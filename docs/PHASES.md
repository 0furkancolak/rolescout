# RoleScout - Development Phases

Status values:
- NOT_STARTED
- IN_PROGRESS
- BLOCKED
- DONE

Only mark a phase DONE after its exit criteria have been verified.

## Phase 0 - Product and Architecture Planning

Status: DONE

Goals:
- Establish product scope and core use cases
- Select the initial stack
- Define service boundaries
- Define engineering and source-integrity principles

Exit criteria:
- Product plan is documented
- Technical boundaries are documented
- Development phases are approved

## Phase 1 - Local Infrastructure and Foundation

Status: DONE

Goals:
- Stabilize Go toolchain configuration
- Complete Go dependency management
- Verify API, worker, and admin builds
- Stabilize Fiber v3 server startup and graceful shutdown
- Verify PostgreSQL connection and migrations
- Complete Dockerfiles and Docker Compose services
- Verify React/Vite build and test commands
- Establish CI checks
- Verify health and readiness endpoints

Tasks:
- [x] Confirm Go uses the intended GOROOT
- [x] Run go mod tidy
- [x] Fix Fiber v3 shutdown compatibility
- [x] Run go test ./...
- [x] Run go vet ./...
- [x] Build every Go entry point
- [x] Build API, worker, and web Docker images
- [x] Start services with Docker Compose
- [x] Verify PostgreSQL readiness
- [x] Verify API health and readiness endpoints
- [x] Verify frontend tests and production build
- [x] Verify CI workflow (GitHub Actions run 39029516196 passed)

Exit criteria:
- All required builds and tests pass
- All required containers start successfully
- API health and readiness checks pass
- PostgreSQL data persists across normal container restarts
- No destructive database reset is required

## Phase 2 - Domain Model and Database

Status: DONE

Goals:
- Define domain entities and relationships
- Implement migration conventions
- Add repositories and database constraints
- Define transaction boundaries

Initial entities to evaluate:
- User
- Workspace
- WorkspaceMembership
- Job
- JobSource
- JobSourceListing
- SavedJob
- Application
- ApplicationAttempt
- ApplicationEvent
- Tag
- Note

Tasks:
- [x] Confirm entity definitions and ownership rules (docs/adr/0001-domain-model-and-schema.md)
- [x] Define schema and indexes
- [x] Add timestamped migrations
- [x] Add constraints and foreign keys
- [x] Add repository tests
- [x] Document important data decisions

Exit criteria:
- Schema supports core use cases
- Constraints protect data integrity
- Migration execution is repeatable
- Repository tests pass

## Phase 3 - API Contracts and Core Backend

Status: DONE

Goals:
- Define REST API conventions
- Publish OpenAPI specification
- Implement validation and consistent error responses
- Implement core job and application use cases

Tasks:
- [x] Define API response and error formats
- [x] Define pagination, filtering, and sorting
- [x] Implement job listing and search endpoints
- [x] Implement saved jobs
- [x] Implement application tracking
- [x] Implement application attempts and history
- [x] Add unit and integration tests
- [x] Document API contracts

Exit criteria:
- Core use cases work through the API
- API contract is documented
- Authorization and validation are tested
- Integration tests pass

Notes:
- Contract: docs/api/openapi.yaml, conventions: docs/api/README.md.
- Validation and workspace isolation are tested. The authorization exit
  criterion was verified with Phase 4's membership-based authorizer.

## Phase 4 - Authentication and Workspace Isolation

Status: DONE

Goals:
- Implement authentication
- Define user and workspace permissions
- Enforce tenant and resource isolation

Tasks:
- [x] Choose and document the authentication approach
- [x] Implement authentication lifecycle
- [x] Implement authorization policies
- [x] Enforce workspace membership
- [x] Test cross-user and cross-workspace access
- [x] Add security-focused tests

Exit criteria:
- Protected endpoints require valid authentication
- Resource access is authorized on the server
- Isolation tests pass

Notes:
- Decision record: docs/adr/0002-authentication.md (email + password,
  Argon2id, server-side sessions; first user may always register).
- Login rate limiting and password reset are deferred (Phase 7 and later).

## Phase 5 - Frontend Application

Status: DONE

Goals:
- Build the responsive Turkish and English application
- Establish feature-based frontend structure
- Integrate API queries and mutations

Tasks:
- [x] Define frontend feature boundaries
- [x] Implement routing and layouts
- [x] Implement job search and filters
- [x] Implement job details and saved jobs
- [x] Implement application tracking views
- [x] Implement application history and notes
- [x] Implement loading, empty, and error states
- [x] Add accessibility and responsive checks
- [x] Add component and feature tests

Exit criteria:
- Core workflows are usable through the UI
- API integration is verified
- Responsive behavior and error states are tested
- Production frontend build succeeds

Notes:
- Structure and conventions: web/README.md.
- Verified against the Compose stack through nginx: registration, search with
  filters and pagination, saving, tracking, attempts, notes and tags. No
  horizontal overflow at 375px width.
- No dependencies were added; shadcn/ui from the plan was not adopted because
  a few Tailwind components cover the current screens.

## Phase 6 - Job Source Ingestion

Status: DONE

Goals:
- Establish a maintainable source adapter interface
- Implement worker scheduling and ingestion
- Normalize and deduplicate source data

Tasks:
- [x] Define the source adapter contract
- [x] Define normalized job representation
- [x] Implement retry and failure handling
- [x] Implement idempotent ingestion
- [x] Review permissions and terms for candidate sources
- [x] Implement the first verified source adapter
- [x] Add ingestion observability and tests

Exit criteria:
- The worker imports valid listings reliably
- Duplicate handling is verified
- Failures can be diagnosed and retried
- The enabled source has passed a permission and access review

Notes:
- Decision record: docs/adr/0003-source-ingestion.md. Operations and rules:
  docs/sources/README.md.
- First adapter: Greenhouse public Job Board API, reviewed in
  docs/sources/greenhouse.md and verified against a real board.
- Sources are managed with the new `admin` CLI (`docker compose run --rm admin`).

## Phase 7 - Quality, Security, and Observability

Status: DONE

Goals:
- Improve production reliability
- Verify security boundaries
- Establish useful operational signals

Tasks:
- [x] Add structured logging and request IDs
- [x] Review secrets and configuration handling
- [x] Review input validation and SQL usage
- [x] Add rate limiting where appropriate
- [x] Add metrics and operational diagnostics
- [x] Test backup and restore procedures
- [x] Add end-to-end smoke tests
- [x] Review dependency and container security

Exit criteria:
- Critical workflows have automated coverage
- Operational failures are diagnosable
- Backup and restore procedures are tested
- Security review findings are addressed or documented

Notes:
- Review and known limitations: docs/security.md.
- API and worker log JSON lines; API requests carry `X-Request-ID`. The API
  serves Prometheus metrics at `/metrics`; ingestion runs are inspected with
  `admin sources runs`.
- Sign-in and sign-up are rate limited per client IP; request bodies must be
  JSON.
- `scripts/smoke.sh` checks a running stack through nginx;
  `scripts/backup.sh` and `scripts/restore.sh` were verified by deleting data
  and restoring it. CI runs both against a fresh Compose stack, plus
  govulncheck and npm audit.
- End-to-end browser tests (Playwright) were not added to avoid a new
  dependency; the UI flows are covered by Testing Library tests and the API
  flows by the smoke test.

## Phase 8 - Release and Self-Hosting

Status: DONE

Goals:
- Make installation repeatable for external users
- Publish release and maintenance documentation

Tasks:
- [x] Document environment variables
- [x] Document Docker Compose installation
- [x] Document upgrades and migrations
- [x] Document backup and restore
- [x] Add production deployment guidance
- [x] Verify clean installation from a fresh checkout
- [x] Prepare release checklist

Exit criteria:
- A new operator can install the application from documentation
- Upgrade and backup procedures are documented and verified
- Release checks pass

Notes:
- Docs: docs/installation.md, configuration.md, deployment.md,
  operations.md, RELEASING.md; CHANGELOG.md prepares 0.1.0.
- Clean installation verified from a fresh clone with a separate Compose
  project and ports, following docs/installation.md: stack healthy, smoke test
  passed, version string reported. The HTTPS override was verified with
  `ROLESCOUT_DOMAIN=localhost` (Caddy internal certificate): HTTPS, HSTS,
  Secure cookies and client addresses forwarded through nginx.
- Backup and restore are verified in CI on every change (e2e job); CI run
  39031914862 passed all jobs.
- Tagging v0.1.0 and publishing a GitHub release are left to the maintainer
  (docs/RELEASING.md).

## Phase 9 - Post-MVP Improvements

Status: IN_PROGRESS (first scope DONE: career page research and UI redesign)

Prioritized by the user on 2026-10-17: companies' own career pages, research
the system does by itself with visible stages, free and comprehensive
discovery, many more jobs, a much better UI. Design:
[docs/superpowers/specs/2026-10-17-career-page-research-design.md](superpowers/specs/2026-10-17-career-page-research-design.md);
plan: [docs/superpowers/plans/2026-10-17-career-page-research.md](superpowers/plans/2026-10-17-career-page-research.md).

Done in this scope: discovery, detection, seven ATS adapters and JSON-LD,
research runs with live stages, scoring, full-text search and facets, job
browser, kanban, companies, settings, dark theme. Evidence: Go and web test
suites, CI, and the live run recorded in [operations](operations.md).

Follow-up scope added 2026-10-18: a modern UI refresh (warm design tokens,
rounded cards, stat cards, score rings; plan milestone 9) is DONE: tokens with a
contrast test, primitives, shell, dashboard with an activity series, job
browser, a no-raw-colours test and a browser check at 1280 px and 375 px in
both themes. Optional local AI enrichment (planned, not started) with a small on-device model on Apple Silicon
(plan milestone 10, [ADR 0005](adr/0005-local-llm-enrichment.md)). See the
addendum in the design spec and the addendum at the end of the plan. Local AI follows the release and ships off by default.

Potential work:
- Advanced search and saved filters
- Notifications and follow-up reminders
- Import/export
- Additional verified job source adapters
- Improved analytics
- Optional integrations
- Performance optimization based on measured bottlenecks

Entry criteria:
- MVP workflows are stable
- Improvements are prioritized using user value and evidence

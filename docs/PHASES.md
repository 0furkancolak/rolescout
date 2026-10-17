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

Status: NOT_STARTED

Goals:
- Build the responsive Turkish and English application
- Establish feature-based frontend structure
- Integrate API queries and mutations

Tasks:
- [ ] Define frontend feature boundaries
- [ ] Implement routing and layouts
- [ ] Implement job search and filters
- [ ] Implement job details and saved jobs
- [x] Implement application tracking views
- [ ] Implement application history and notes
- [ ] Implement loading, empty, and error states
- [ ] Add accessibility and responsive checks
- [ ] Add component and feature tests

Exit criteria:
- Core workflows are usable through the UI
- API integration is verified
- Responsive behavior and error states are tested
- Production frontend build succeeds

## Phase 6 - Job Source Ingestion

Status: NOT_STARTED

Goals:
- Establish a maintainable source adapter interface
- Implement worker scheduling and ingestion
- Normalize and deduplicate source data

Tasks:
- [ ] Define the source adapter contract
- [ ] Define normalized job representation
- [ ] Implement retry and failure handling
- [ ] Implement idempotent ingestion
- [ ] Review permissions and terms for candidate sources
- [ ] Implement the first verified source adapter
- [ ] Add ingestion observability and tests

Exit criteria:
- The worker imports valid listings reliably
- Duplicate handling is verified
- Failures can be diagnosed and retried
- The enabled source has passed a permission and access review

## Phase 7 - Quality, Security, and Observability

Status: NOT_STARTED

Goals:
- Improve production reliability
- Verify security boundaries
- Establish useful operational signals

Tasks:
- [ ] Add structured logging and request IDs
- [ ] Review secrets and configuration handling
- [ ] Review input validation and SQL usage
- [ ] Add rate limiting where appropriate
- [ ] Add metrics and operational diagnostics
- [ ] Test backup and restore procedures
- [ ] Add end-to-end smoke tests
- [ ] Review dependency and container security

Exit criteria:
- Critical workflows have automated coverage
- Operational failures are diagnosable
- Backup and restore procedures are tested
- Security review findings are addressed or documented

## Phase 8 - Release and Self-Hosting

Status: NOT_STARTED

Goals:
- Make installation repeatable for external users
- Publish release and maintenance documentation

Tasks:
- [ ] Document environment variables
- [ ] Document Docker Compose installation
- [ ] Document upgrades and migrations
- [ ] Document backup and restore
- [ ] Add production deployment guidance
- [ ] Verify clean installation from a fresh checkout
- [ ] Prepare release checklist

Exit criteria:
- A new operator can install the application from documentation
- Upgrade and backup procedures are documented and verified
- Release checks pass

## Phase 9 - Post-MVP Improvements

Status: NOT_STARTED

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

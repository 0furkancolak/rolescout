# RoleScout - Product and Technical Plan

## 1. Product Vision

RoleScout is an open-source, self-hosted job discovery and application tracking platform.

The goal is to help individuals and teams discover relevant job opportunities, organize applications, track application history, and operate their own deployment with control over their data.

Project name: RoleScout
License: Apache-2.0
Languages: Turkish and English
Deployment: Self-hosted, with support for multi-user installations

## 2. Product Principles

- Open-source and self-hostable
- Low operational cost
- Secure defaults and privacy-conscious data handling
- Modular architecture without unnecessary abstractions
- Reliable job source integrations
- Accessible, responsive user interface
- Observable services and predictable deployments
- Automated tests and repeatable development workflows

## 3. Core Features

### Job Discovery
- Search and filter job listings
- Filter by location, work arrangement, employment type, seniority, and technology
- Search across supported job sources
- Detect duplicate listings
- Track listing freshness and source metadata
- Preserve original listing URLs and attribution

### Job Management
- Save and bookmark opportunities
- Track application status
- Record multiple application attempts for one job
- Keep application history and notes
- Track deadlines, interviews, and follow-up dates

### User and Workspace Management
- Authentication and authorization
- Personal workspace
- Multi-user deployment support
- Data isolation between users and workspaces
- User preferences and saved filters

### Source Ingestion
- Independent background worker
- Source-specific adapters
- Scheduled ingestion
- Retry and failure handling
- Deduplication and normalization
- Source terms, permissions, and access requirements verified before enabling each adapter

### Operations
- Health and readiness endpoints
- Structured logging
- Database migrations
- Containerized deployment
- CI checks
- Backup and recovery documentation

## 4. Technology Stack

### Backend
- Go
- Fiber v3
- PostgreSQL
- pgx
- REST API
- OpenAPI
- Separate API, worker, and admin entry points

### Frontend
- React
- TypeScript
- Vite
- Feature-based architecture
- React Router
- TanStack Query
- Tailwind CSS
- shadcn/ui
- Zod
- React Hook Form
- Vitest
- React Testing Library
- Playwright for end-to-end testing

### Infrastructure
- Docker Compose
- PostgreSQL persistent volume
- Environment-based configuration
- CI pipeline
- Structured application logs

## 5. Architecture Boundaries

### API
Responsible for HTTP requests, authentication, authorization, validation, application use cases, and API responses.

### Worker
Responsible for scheduled ingestion, source synchronization, retries, and background processing.

### Admin
Responsible for operational and administrative workflows. Admin capabilities must be authorized and audited where appropriate.

### Web
Responsible for user-facing screens, navigation, forms, data fetching, and interaction states.

### PostgreSQL
The primary source of truth for users, workspaces, jobs, sources, saved jobs, applications, and application history.

## 6. Engineering Rules

- Keep business rules independent of HTTP handlers.
- Keep database access behind explicit infrastructure boundaries.
- Use context cancellation and graceful shutdown.
- Validate all external input.
- Use parameterized SQL.
- Enforce authorization on the server.
- Never commit secrets or production credentials.
- Use additive, versioned database migrations.
- Use UTC for persisted timestamps and explicit time-zone handling for display.
- Add indexes based on query patterns and measured needs.
- Avoid premature microservices, speculative abstractions, and unnecessary dependencies.
- Add tests for meaningful business rules and failure scenarios.
- Document architecture decisions that materially affect future implementation.

## 7. Data and Source Integrity

- Store source identifiers and original URLs.
- Normalize source data before persistence.
- Use stable deduplication keys where possible.
- Track ingestion time and source update time separately.
- Preserve application attempt history instead of overwriting prior attempts.
- Do not bypass source authentication, access controls, or applicable terms.
- Enable each real-world source only after its access method and permissions have been reviewed.

## 8. Definition of Done

A feature is complete when:
- Acceptance criteria are met.
- Relevant tests pass.
- Formatting, linting, and static analysis pass.
- Database changes include migrations when required.
- API behavior and configuration are documented.
- Error handling and authorization are implemented where applicable.
- The feature works in the intended local deployment.
- No secrets or unrelated files are included in the change.

## 9. Current Implementation Status

The repository has a Go API, worker, configuration and database packages, a React/Vite frontend, and PostgreSQL-based local infrastructure.

The current setup still requires validation of:
- Fiber v3 server shutdown handling
- Go tests and builds for every entry point
- Docker image builds with a compatible Go toolchain
- Compose service startup and health/readiness checks
- Frontend tests and production build
- End-to-end local smoke tests

Do not mark these items complete until their commands succeed.

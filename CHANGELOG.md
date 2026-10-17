# Changelog

All notable changes are listed here. Versions follow
[Semantic Versioning](https://semver.org/); before 1.0, minor versions may
include breaking changes, which are called out.

## [Unreleased]

## [0.1.0] - 2026-10-17

First release.

### Added
- Job discovery: search by text, work arrangement, employment type, seniority
  and technologies, with keyset pagination; job details with links to every
  source listing; saved jobs.
- Application tracking: one application per job and workspace with a status
  workflow, numbered application attempts, an append-only history,
  interviews, follow-ups and deadlines, notes and tags.
- Accounts and workspaces: email and password sign-in with server-side
  sessions; personal and shared workspaces with owner and member roles; strict
  isolation between workspaces.
- Web app in Turkish and English, usable on phones and desktops.
- Source ingestion: worker with scheduling, retries and run history; the
  Greenhouse public Job Board API adapter; cross-source deduplication; an
  `admin` CLI to add, review, enable and sync sources.
- Operations: Docker Compose setup with health checks, JSON logs with request
  IDs, Prometheus metrics, sign-in rate limits, backup, restore and smoke test
  scripts, and an HTTPS override with Caddy.

[Unreleased]: https://github.com/0furkancolak/rolescout/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/0furkancolak/rolescout/releases/tag/v0.1.0

# Changelog

All notable changes are listed here. Versions follow
[Semantic Versioning](https://semver.org/); before 1.0, minor versions may
include breaking changes, which are called out.

## [Unreleased]
### Added
- Regional search: filter jobs by place (`location`, ignoring case and
  diacritics), by region (`region`: EU, EMEA, DACH…) and by job board system
  (`ats`, such as Lever); a "Türkiye" quick filter; countries in research
  profiles can be typed by name in English or Turkish. Match scores know
  regions ("Remote - EMEA") and rate remote jobs open only to another country
  lower (`remote_abroad`).
- Türkiye coverage: all 81 provinces and main districts, US states and more
  places are recognised (`admin countries backfill` fixes stored jobs); a
  second seed list of employers with open jobs in Türkiye; a discovery source
  that finds Lever, Greenhouse, Ashby, Workable and Recruitee boards from
  company names (`internal/discovery/seeds/probe_names.txt`).
- Lever postings with several sites keep all of them in the location.
- Retry delays for match explanations that keep failing.
- `Makefile` with `make dev` (backend in Docker, web app with hot reload) and
  `up`, `down`, `logs`, `test`, `lint` and `smoke` tasks.
- `scripts/init-env.sh` creates `.env` with a random database password.
- Career page research: research profiles (a "Full Stack Developer" profile
  for every workspace) and research runs with five visible stages —
  discovery, detection, collection, scoring and result — and a live log.
- Company discovery from Hacker News "Who is hiring?" threads, public company
  lists, a verified seed list and links between career pages; career page
  detection for embedded boards, redirects and schema.org JobPosting data.
- Adapters for Lever, Ashby, Workable, Recruitee, Personio, BambooHR, JSON-LD
  career pages and Workday (off until reviewed).
- Polite crawler: robots.txt with `Crawl-delay`, per-host and per-vendor
  limits, identifying User-Agent, response limits.
- Explainable match scores, full-text search, counted filters (posted date,
  arrangement, technology, country, company, seniority) and a three-column
  job browser with keyboard navigation.
- Applications kanban board, company catalog with per-workspace hiding,
  dashboard, administrator settings, dark theme.
- Modern interface: warm light and matching dark theme built on tested design
  tokens (WCAG contrast checked), floating pill navigation, account menu,
  company tiles, match-score rings, dashboard stat cards with sparklines
  (`GET /workspaces/{id}/activity`), quick filter chips and a segmented sort
  control in the job browser.
- Closed-job tracking and retention (`CLOSED_JOB_RETENTION`), tiered refresh
  of company sources, `admin adapters`, `admin companies` and
  `admin seeds check` commands.

- Optional local AI enrichment (`LLM_ENABLED`, off by default): a small model on
  your own computer (Qwen3 through Ollama or llama.cpp) reads job descriptions
  in a separate worker loop, with grounded and validated output, a circuit
  breaker, a description-hash cache, AI-marked salary, remote policy, summary and
  technologies, one-line match explanations, Settings → Local AI, and
  `admin llm check|eval|enrich`. See docs/local-ai.md and ADR 0005.
- Labelled evaluation set (30 English and 30 Turkish postings) and
  `scripts/llm-bench.sh` / `scripts/llm-eval.sh` to measure a model.

### Changed
- The model's work arrangement and country must be supported by the text;
  technology names that are also ordinary words ("Go", "Spring") need
  technology wording around them.
- The global job search and the enrichment queue count use the same rules as
  the workspace browser (effective fields; current prompt version).
- Source reviews are recorded per adapter type (ADR 0004); a source can be
  enabled when its adapter type is reviewed and enabled.
- The worker runs sources in parallel (`CRAWL_CONCURRENCY`).
- The first account is the installation administrator.
- Screens use semantic colour tokens only (a test forbids raw colours); the
  old slate-remapping dark mode block is gone.
- Signing in opens the dashboard; `/jobs/{id}` links open the job browser.

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

# Changelog

All notable changes are listed here. Versions follow
[Semantic Versioning](https://semver.org/); before 1.0, minor versions may
include breaking changes, which are called out.

## [Unreleased]
### Added
- Settings is split into tabs (Job boards, People, AI, Crawl); the tab is kept
  in the address (`/settings?tab=ai`).
- "Draft with AI" on a new research profile: describe the job in your own words
  and the AI fills the profile form to review and save
  (`POST /workspaces/{id}/research-profiles/suggest`, nothing is saved by it).
- Single-install accounts, like Grafana: only the first account can sign up;
  after that the "create account" page and link disappear and the administrator
  adds people in Settings → People (`GET`/`POST /admin/users`,
  `GET /auth/setup`). `ALLOW_REGISTRATION=true` still opens sign-up for everyone.
- Newer OpenAI models (`gpt-5`+, o-series) work: `max_completion_tokens` with
  reasoning headroom, no custom temperature, learned on the fly for other names.
- AI with an API key: Settings → AI edits the connection (on/off, base URL,
  model, API key) in the database; changes apply within seconds without a
  restart and override `LLM_*` (`GET`/`PUT /admin/llm/config`). The key is never
  returned. Works with any OpenAI-compatible service.
- "Generate AI explanation" on a job's "Why it matches": reads the posting and
  writes why it fits at any score, so low-scoring matches get AI text on demand
  (`POST /workspaces/{id}/jobs/{id}/analyze`, 60 an hour per client). For a
  hosted service the worker no longer reads the whole catalog unless the
  administrator turns on background reading.
- `make doctor` checks Docker, Node, ports and `.env`; `make dev` stops early
  with a clear message (for example when port 8080 is taken); `make rebuild`
  restarts the API and worker after Go changes; `make test-api` runs the
  database tests against the dev PostgreSQL when it is up.
- Job browser: results load as you scroll; filters moved to a drawer behind a
  "Filters" button; the selected job scrolls on its own with its title and
  actions pinned; original description on the left and the model's reading
  (summary in the interface language, remote policy, why it matches) on the
  right; "Başvurdum" can be undone in place.
- A short summary of each job written by the local model in the interface
  language on first view and stored (`GET /workspaces/{id}/jobs/{id}/summary`).
  The text streams into the page as the model writes it, as server-sent events
  (`.../summary/stream`); several viewers of one job share one model run.
- The job browser remembers where you left off (filters, sort, selected job)
  in the database and continues there on any device
  (`GET`/`PUT /workspaces/{id}/state/{key}`).
- Application details page and a bookmarklet that fills the common fields of a
  career page's application form (never submits, never fills files).
- Applications can be deleted (`DELETE …/applications/{id}`); the list and the
  board were redesigned (open/closed groups, status chosen in place).
- Company discovery also looks for the job boards of catalogued companies that
  have no readable career page (name probe over thousands of Hacker News
  employers).
- `make dev` streams the API and worker logs into the terminal.
### Changed
- No favicon.
- The header shows the app name only, without a logo icon.
- PostgreSQL runs with a bounded cache, 40 connections and no parallel workers
  (about 20 MB idle) instead of the image defaults; the API write timeout is
  3 minutes so a summary can stream.
- Any application status can change to any other, so a mistaken one can be
  corrected.
- Selects have room for their arrow; scrollbars follow the theme.

## [0.2.0] - 2026-10-19
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

[Unreleased]: https://github.com/0furkancolak/rolescout/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/0furkancolak/rolescout/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/0furkancolak/rolescout/releases/tag/v0.1.0

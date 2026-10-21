# Changelog

All notable changes are listed here. Versions follow
[Semantic Versioning](https://semver.org/); before 1.0, minor versions may
include breaking changes, which are called out.

## [Unreleased]
### Fixed
- AI summaries: postings without a description (kariyer.net lists) now get a
  line built from the listing's own fields instead of "unavailable"; a summary
  the model got wrong is retried once and then replaced by the posting's opening
  sentences instead of being dropped; the model gets room for two or three full
  sentences (Turkish text was cut short) and skips boilerplate. Old summaries
  are rewritten (prompt version 4).
- Career pages that print an emoji beside each job no longer get that emoji as
  the job's location (which hid such jobs from country filters).
### Added
- More company job-board systems: Teamtailor, Breezy HR, Rippling and Gem
  (read, detected on career pages, probed by company name).
- Job sites that list many employers, through public APIs and feeds: Remote OK,
  Arbeitnow, Himalayas, Jobicy, The Muse, We Work Remotely, Working Nomads and
  Hacker News "Who is hiring?" (29 default sources). Sites whose robots.txt or
  terms forbid it are listed with reasons in docs/sources/README.md.
- Discovery: about 420 more employer sites (Turkish corporates, staffing and
  recruitment agencies, software houses, consultancies), Y Combinator companies
  that are hiring, DBpedia read by country category and for recruitment/IT
  agencies (Turkey from about 200 to about 980 companies), more company lists,
  and one company per site however its address is spelled.

### Fixed
- DBpedia companies were not stored because the `dbpedia` origin was missing
  from the database constraint.

### Added
- Career pages that are plain HTML with no job board or JSON-LD (such as
  e-adam.com/kariyer) are read by the new `htmlpage` adapter; detection also
  tries the usual career paths (`/kariyer`, `/careers`, `/is-ilanlari`, ...)
  on a company's home page.
- `kariyernet` adapter: reads the public category lists of kariyer.net
  (title, company, place, work model) every 12 hours. See docs/sources/kariyernet.md
  for its limits; it can be switched off with `admin adapters disable`.
- Discovery adds DBpedia as a source of companies with websites per country
  (the profile's countries first).
- AI memory: a note about the work you look for and an uploaded CV (PDF, Word
  or text, 5 MB) per workspace. Profile drafts, match explanations and the
  technology comparison use them; a draft can be made from the memory alone.
  Profile drafts are fuller (up to 25 titles, 30 keywords and technologies, more
  locations and exclusions, local-language titles).
- Jobs compare their technologies with the profile and the CV: technologies
  you have are green, those you lack are red, those you excluded are struck
  through; match explanations mention the gaps.
- `RESEARCH_PARALLEL` (default 3): that many research runs execute at the same
  time. The shared stages (discovery, detection, collection) still go one run
  at a time so no site is read twice.
- Updates no longer lose a research run: when the worker is stopped (an update,
  a restart) the run in progress goes back in the queue and starts again, and
  the worker gets 60 s to do so. `ROLESCOUT_SERVICES="api web"` lets
  `scripts/deploy-server.sh` update only some services.
- Research profiles can have automatic runs switched off (`schedule_enabled`);
  the research screen shows when the next automatic run is due.
- Application details page shows the job's company, place, work arrangement,
  seniority, first application and a link to the original posting.
- Autofill bookmark can be copied as code with steps, for browsers where
  dragging it to the bookmarks bar does not work.
- Country focus: a research profile's countries are read first in every stage
  (companies of those countries are identified and their jobs collected before
  the rest), and the sector search asks about those countries before the
  general regions. Companies carry a country (from the career page's domain, the
  shipped Türkiye list, then the countries of their jobs).
- Company discovery from more places: US stock exchange listings (Nasdaq Trader
  directories) and, when AI is on, sector by sector (25 industries × worldwide,
  Türkiye, Europe, United States). Both only produce names; a name becomes a
  company only if its job board really exists (the name probe). They work in
  batches over several runs instead of blocking one.
- Settings → System: RAM each service uses now and may use (API and worker
  measured, PostgreSQL and web by limit), with container memory limits in
  Compose (`API_MEM_LIMIT`, `WORKER_MEM_LIMIT`, `POSTGRES_MEM_LIMIT`,
  `WEB_MEM_LIMIT`) and `GET /admin/system`.
- Research run history shows duration, counts and errors per run; a run page
  shows when it was queued, started and finished, each stage's start and
  duration, and a log you can filter by stage.
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
- The starter profile of a new account no longer runs automatically; switch
  auto-run on when you want it.
- Application board: cards move with step buttons instead of drag and drop (which
  did not work on touch screens and some browsers); research run history shows
  its counts as tiles; hover and focus states on cards, rows and buttons.
- Research is much faster: all discovery sources run at the same time (the
  stage shows sources finished), names are probed eight at a time, defaults are
  32 hosts in parallel and 8 requests per job board API (150 ms apart), the
  sector search asks four sector/region questions at once, and larger career
  sites are read in full (no page cap: every page found is read, Workday until its list ends).
- Research speed is a setting: Settings → Crawl lets an administrator change how many
  companies are worked on at once and the gaps between requests (to one site and to one
  job board); the worker follows within seconds, no restart. Defaults are higher: 128
  companies at once (was 32), 500 ms per site (was 1 s), 16 requests per job board 100 ms
  apart (was 8, 150 ms). New variable `CRAWL_VENDOR_INTERVAL`.
- Detection takes larger batches when more companies run at once, so workers are not left
  waiting for the slowest company of a batch.
- All SQL is written out as literal statements with parameters; the dynamic
  query builder (job search, facets) is replaced by fixed statements with
  optional filters, and a test fails if SQL is ever built at run time.
- Hacker News discovery reads a whole "Who is hiring?" thread in one request
  (HN Search API) instead of one request per comment, so the first run takes
  seconds instead of tens of minutes; the old per-comment path is the fallback.
- A run that failed, was cancelled or was interrupted no longer shows a stage as
  still "in progress" (stored that way too); its log is shown when you open the
  page again; the run page shows memory use for administrators.
- Every clickable control shows a pointer cursor; dialogs dim the page with a dark
  scrim in both themes (it looked like a white wash in dark mode).
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

# Career Page Research and UI Redesign: Design

Date: 2026-10-17
Status: Draft for approval (builder check done)

## Intent
Today RoleScout only imports jobs from sources an operator adds and reviews by
hand, one Greenhouse board at a time. The database is empty after install and
the user sees nothing until they do CLI work. The user, a full stack
developer looking for work, wants the opposite: the system should go out on
its own, find companies' own career pages, collect a very large number of
jobs from them, show each stage of that research while it runs, and present
the results in a much better interface.

Priorities, in the user's words and choices:
1. Companies' own career pages are the most important source.
2. The system searches by itself and shows its stages.
3. Research must be comprehensive and must be free (no paid APIs, no API keys).
4. The UI must be much better. The research screen uses vertical stages with
   a live log (option A); the job screen uses filters, list and detail side by
   side (option A).
5. Defaults are left to the builder; the default role is Full Stack Developer.

## Goals
- Discover thousands of companies with public career pages without any API
  key or paid service, and keep the list growing over time.
- Collect every open job from those career pages through their public job
  APIs or structured data, refreshed on a schedule.
- Let a user define research profiles (role, keywords, technologies,
  locations, remote preference, seniority) and score every job against them
  with explainable reasons.
- Show a research run's stages, counts and a live log while it runs.
- Redesign the UI: dashboard, research, three-column job browser, kanban
  applications board, companies page, dark mode.

## Non-goals
- Scraping LinkedIn, Indeed, Kariyer.net, Glassdoor or other job aggregators
  whose terms forbid automated access.
- Paid or keyed services (search APIs, LLM APIs). They may come later as
  optional add-ons, not in this design.
- Applying to jobs automatically, emailing recruiters, notifications by email.
- Headless browser rendering of JavaScript-only career pages.

## Anti-goals (failure even if it works)
- Ignoring `robots.txt`, hammering a host, or hiding who we are. Every request
  identifies RoleScout and respects per-host limits.
- A research screen that looks busy but hides failures. Failed companies and
  their reasons must be visible.
- Scores without reasons. Every match explains why it scored as it did.
- Losing the existing data model guarantees: workspace isolation, dedupe,
  idempotent upserts, and tests per layer.

## Constraints
- Stack stays as is: Go (Fiber v3, pgx), PostgreSQL 17, React 19 + Vite +
  TanStack Query + Tailwind, Docker Compose. New third-party dependencies need
  the user's approval before they are added. The user approved promoting
  `golang.org/x/net` (already an indirect dependency) to a direct one for its
  `html` parser, used for ATS embed detection, link following and JSON-LD.
  Nothing else is approved.
- Tests: Go tests beside code, web tests in `__tests__` folders.
- Bilingual UI (en reference, tr same keys).
- Must run on one machine; the full crawl must finish in hours, not days, and
  must not need more than a normal laptop's resources.

## Decided by the user
- Career pages first; aggregators dropped from this scope.
- Free discovery only.
- Research UI: vertical stage cards with progress bars and counts, and a live
  log beside them (mockup A).
- Job UI: counted filters on the left, scored list in the middle, full detail
  with "why it matches" on the right (mockup A).
- Remaining UI (dashboard, kanban, companies page, dark mode, keyboard
  navigation) as proposed in chat and accepted ("Ok go"); there are no mockups
  for these, so their visual design is the builder's.
- Mockups A for research and jobs are in
  `.superpowers/brainstorm/*/content/research-layout.html` and
  `results-layout.html` (git-ignored, local reference only).
- Closed jobs that nobody saved or applied to are deleted 90 days after they
  close (`CLOSED_JOB_RETENTION=2160h`, `0` keeps them forever). Saved jobs and
  jobs with applications are never deleted.

## How it works

### Concepts
- **Company**: an employer with a career page. Global (not per workspace).
  Fields: name, website domain (if known), career page URL, detected ATS
  (`greenhouse`, `lever`, `ashby`, `workable`, `smartrecruiters`, `recruitee`,
  `personio`, `bamboohr`, `jsonld`, `workday`, or `unknown`), ATS identifier
  (board token / slug), discovery origin, status (`active`, `empty`,
  `unreachable`, `blocked_by_robots`, `disabled`), last check result.
- **Job source**: unchanged concept. Each company with a detected ATS gets one
  `job_sources` row (adapter + config). Existing sync, runs and dedupe code is
  reused.
- **Research profile**: per workspace. Name, title keywords, extra keywords,
  technologies, locations/countries, remote preference, seniority list,
  excluded keywords, excluded companies. A default "Full Stack Developer"
  profile is created for every new workspace and for existing workspaces by
  migration.
- **Research run**: one execution of the pipeline for a profile, with stage
  states, counters and a log.
- **Match**: a job's score (0–100) for a profile, with stored reasons.

### Pipeline stages
A research run moves through five stages. Stages 1–3 work on the global
company catalog and are shared; stage 4 is per profile.

1. **Company discovery.** Grows the company catalog from free sources:
   - *Common Crawl URL index* (`index.commoncrawl.org`, CDX API). For each ATS
     host pattern (for example `boards.greenhouse.io/*`,
     `job-boards.greenhouse.io/*`, `jobs.lever.co/*`, `jobs.ashbyhq.com/*`,
     `apply.workable.com/*`, `jobs.smartrecruiters.com/*`, `*.recruitee.com`,
     `*.jobs.personio.de`, `*.bamboohr.com/careers`), query the latest crawl
     and extract distinct board identifiers from the URLs. Paged, one request
     at a time, with a pause between pages, results cached so later runs only
     read new crawls.
   - *Seed list* shipped in the repo (`api/internal/discovery/seeds/*.txt`):
     career page URLs of well-known tech employers, including Turkish
     companies, for coverage Common Crawl misses. Each entry is verified when
     the list is built; dead ones are dropped.
   - *Link following*: career pages and JSON-LD `hiringOrganization.sameAs`
     links that point to other ATS boards add those companies.
   - *Manual add*: a user pastes a career page URL in the UI or CLI.
   Discovery only runs if the last full discovery is older than a configurable
   interval (default 7 days); otherwise the stage completes immediately with
   "catalog is fresh, N companies".
2. **Career page detection.** For each new or unknown company, determine the
   ATS and identifier:
   - URL pattern match for known ATS hosts (no request needed).
   - For other URLs: check `robots.txt`, fetch the page, look for ATS embed
     scripts/iframes/links (Greenhouse embed, Lever, Ashby, Workable widget,
     etc.), else look for schema.org `JobPosting` JSON-LD on the page or pages
     it links to under the same path (`jsonld`), else `unknown`.
   - Verify by calling the ATS public API once: board exists and returns a
     valid response. Zero jobs is `empty`, not a failure.
   - Creates or updates the company's `job_sources` row.
3. **Job collection.** Syncs every active company source that is due (default
   interval 12 hours; `empty` companies 3 days) using the existing runner and
   new adapters. Listings missing from a successful snapshot are marked closed
   (see Data model). Concurrency is bounded per host and globally (see Polite
   crawling). Refresh is tiered so the catalog can grow without making a pass
   take days: companies with at least one job matching any profile refresh
   every 12 hours, companies with open jobs but no matches every 3 days,
   `empty` companies every 7 days. The detection check call for an ATS board
   is reused as its first sync so new boards cost one request, not two.
4. **Matching and scoring.** Scores every open job against the run's profile
   (see Scoring) and stores matches above a minimum (default 40). Jobs first
   seen since the profile's previous run are flagged new.
5. **Result.** Summary: companies known, checked, failed (with reasons
   grouped), jobs collected, open jobs, matches, new matches. Kept with the
   run for history.

Runs are executed by the worker. The API inserts a `queued` run; the worker
claims it with `FOR UPDATE SKIP LOCKED`. Only one run executes stages 1–3 at a
time across the instance; a second run queued meanwhile waits and then reuses
the fresh results (its stages 1–3 complete quickly). Each profile also runs on
a schedule (default every 12 hours) so results stay fresh without a click.
A run can be cancelled; cancellation stops at the next company boundary.

### Adapters
Existing `ingest.Adapter` contract is kept. New adapters, each using only the
vendor's public, unauthenticated job board endpoint intended for embedding job
lists on company sites:
- `lever`: `api.lever.co/v0/postings/{site}?mode=json`
- `ashby`: `api.ashbyhq.com/posting-api/job-board/{board}?includeCompensation=true`
- `workable`: `apply.workable.com/api/v1/widget/accounts/{account}`
- `smartrecruiters`: `api.smartrecruiters.com/v1/companies/{id}/postings` (paged)
- `recruitee`: `{company}.recruitee.com/api/offers/`
- `personio`: `{company}.jobs.personio.de/xml`
- `bamboohr`: `{company}.bamboohr.com/careers/list`
- `jsonld`: fetches the career page (and same-site job pages it links to, up to
  a limit, default 200 pages per company), extracts `JobPosting` JSON-LD.
- `workday`: uses the JSON endpoint behind Workday career sites
  (`{tenant}.wd{n}.myworkdayjobs.com/wday/cxs/{tenant}/{site}/jobs`). It is not
  a documented public API, so this adapter ships **disabled**: the operator
  must review and enable the `workday` adapter type explicitly.

Each adapter's exact endpoint behaviour and terms are verified at build time
and documented in `docs/sources/<adapter>.md` like Greenhouse today.

### Review model change
Today every source needs its own review before it can be enabled. With
thousands of discovered companies that blocks the product. New rule:
- Review moves to the **adapter type** (`source_adapter_reviews` table: adapter,
  reviewed_at, note, enabled). A source can be enabled only if its adapter
  type is reviewed and enabled. The database constraint is replaced with a
  trigger or FK-backed check enforcing this.
- The shipped migration records reviews for the public job board APIs
  (Greenhouse, Lever, Ashby, Workable, SmartRecruiters, Recruitee, Personio,
  BambooHR) and `jsonld`, with notes linking to the terms checked at build
  time. `workday` is not reviewed by default.
- Per-source disable still exists (companies page, CLI).
- Existing manually reviewed sources keep working.
- Written up as ADR 0004, superseding the review part of ADR 0003.

### Polite crawling
- Shared HTTP client: User-Agent `RoleScout/<version> (+<project URL>)`,
  timeouts, response size limit, redirects limited to http(s).
- `robots.txt` fetched and cached per host (24 h); disallowed paths are skipped
  and reported as `blocked_by_robots`. ATS JSON API hosts are checked too.
- Per-host limits: career sites and other hosts get 1 request at a time,
  at least 1 second apart. Documented vendor job board API hosts (one host
  serves every company on that vendor) get up to 4 requests at a time, at
  least 250 ms apart per connection. Global concurrency default 16. Design
  target: a catalog of about 20 000 companies and up to 500 000 open jobs; a
  full first pass of 20 000 boards then takes roughly 1–2 hours. `429` and
  `Retry-After` are honoured; repeated failures back off per company
  (exponential, capped at 7 days).
- Conditional requests (`ETag` / `If-Modified-Since`) where the host supports
  them.
- All limits are configuration variables with the defaults above.

### Scoring
Deterministic and explainable; no ML. For a profile and a job:
- Title match (up to 40): profile title keywords and their known variants
  ("full stack", "full-stack", "fullstack", "full stack engineer/developer",
  "software engineer, full stack", "product engineer"…) in the job title.
- Technology overlap (up to 30): share of the profile's technologies found in
  the job's technologies.
- Location and arrangement (up to 15): remote preference and countries/cities.
- Seniority (up to 10): job seniority in the profile's list, or unknown.
- Freshness (up to 5): first seen within 7 days.
- Hard filters: excluded keywords or companies, or a remote-only profile with
  an onsite job outside the profile's locations, remove the job.
Reasons are stored as a list of short codes with parameters (for example
`title:full stack`, `tech:5/6`, `remote`), rendered translated in the UI.
Weights live in one Go file with tests; exact keyword lists are the builder's.

### Search
- Add a full-text `tsvector` column on jobs (title weight A, company and
  technologies B, description C; `simple` configuration so English and
  Turkish both work) with a GIN index, replacing `ILIKE` search.
- Job browser queries combine: profile matches (sort by score) or plain search
  (sort by rank, then recency), and filters. Filter facets return counts for
  posted date, arrangement, technology (top 20), country, company (top 20),
  seniority.
- Location gets a normalized `country_code` where it can be inferred (from
  ATS structured fields, JSON-LD `addressCountry`, or a city/country lookup
  table); unknown stays null.

### Data model changes (summary)
- `companies` (new), `job_sources.company_id` (nullable for legacy sources).
- `source_adapter_reviews` (new), constraint change on `job_sources`.
- `jobs`: `search` tsvector, `country_code`, `company_id` (nullable),
  `closed_at` (null while open). A job is closed when none of its listings
  appeared in their source's latest successful snapshot; it reopens if seen
  again. Closed jobs are hidden from search by default but kept for saved jobs
  and applications.
- `research_profiles`, `research_runs` (status, stage, per-stage
  state/counters JSON, timestamps, error), `research_run_events` (run, time,
  level, stage, company, message; capped at the latest 2 000 per run),
  `research_matches` (profile, job, score, reasons, first_matched_at, new
  flag), `workspace_company_blocks`.
- Instance admin: `users.is_admin`, true for the first user. Only admins can
  review adapter types, disable companies globally and change crawl settings.
- Sharing on multi-user instances (builder's call, RoleScout is mainly
  single-user): the company catalog is global public information; a manually
  added company joins the global catalog and records no visible adder. Any
  user can start a run; stages 1–3 are shared and their log entries mention
  companies only, never profiles or workspaces. Profiles, matches, blocks and
  everything in stage 4 stay workspace-private.
- Retention: a daily worker task deletes closed jobs past
  `CLOSED_JOB_RETENTION` unless saved or linked to an application, and run
  events older than 30 days.

### API (new endpoints, under the existing conventions)
- Research profiles: CRUD under `/workspaces/{id}/research-profiles`.
- Runs: start, list, get (stages, counters), events since cursor, cancel.
- Matches and job browsing: the job search endpoint gains `profile`, facet
  and new filters; facets endpoint returns counts.
- Companies: list with filters and status, add by career page URL, block per
  workspace; admin: disable globally.
- Admin: adapter reviews list/update, crawl settings read.
- Applications: status change endpoint used by the kanban (exists).
OpenAPI contract updated for all of them.

### Live progress
The UI polls the run every 2 seconds while it is running (run summary plus
events after the last seen event ID), and stops when it finishes. No
WebSocket or SSE.

### UI
- **Layout**: left sidebar navigation on desktop (Dashboard, Research, Jobs,
  Saved, Applications, Companies, Settings), bottom-collapsing menu on mobile.
  Refreshed visual style; dark mode follows the system with a manual toggle
  remembered per browser.
- **Dashboard**: last run summary and button to run now, new matches today,
  application counts by status, upcoming interviews and follow-ups.
- **Research**: profile list and editor; run screen as mockup A: five vertical
  stage cards (state icon, progress bar, counts, per-ATS breakdown for
  detection, failure count) and a live log beside it, with filters for errors
  only. Run history with summaries.
- **Jobs**: mockup A. Search box; left: facets with counts; middle: scored
  list with "new" badges and infinite scroll; right: detail with title,
  company, location, arrangement, technologies, "why it matches", Save,
  "I applied" (creates an application), open career page. Keyboard: `j`/`k`
  move, `s` save, `o` open. On mobile the detail opens as a full page.
  The selected job and filters live in the URL.
- **Applications**: list view (exists) plus kanban board by status with native
  HTML drag and drop and keyboard alternative (move via menu); invalid status
  transitions are refused with a message.
- **Companies**: catalog table with ATS, status, open job count, last check,
  failure reason; add by URL; block for this workspace.
- **Settings** (admin): adapter types with review state, crawl limits shown.

The builder owns visual details, component structure and copy, within the
existing frontend conventions (`web/README.md`).

## Configuration (new variables, defaults)
`CRAWL_CONCURRENCY=16`, `CRAWL_HOST_INTERVAL=1s`, `CRAWL_USER_AGENT_URL=<repo
URL>`, `DISCOVERY_INTERVAL=168h`, `COMPANY_SYNC_INTERVAL=12h`,
`RESEARCH_INTERVAL=12h`, `JSONLD_MAX_PAGES=200`, `MATCH_MIN_SCORE=40`,
`CLOSED_JOB_RETENTION=2160h`, `CRAWL_VENDOR_CONCURRENCY=4`.

## Testing
- Adapters: recorded fixtures per vendor; no live calls in unit tests.
- Discovery: CDX parsing and board extraction from fixture responses; seed
  list format test.
- Detection: HTML fixtures for each embed type and JSON-LD.
- Polite client: robots.txt rules, per-host limiting, Retry-After.
- Scoring: table tests for each component and hard filters.
- Pipeline: integration test with a fake HTTP server standing in for Common
  Crawl and ATS hosts, asserting stage transitions, counts, events, closed
  jobs and resumability.
- Web: research run screen (polling and stage rendering), job browser
  (facets, selection, keyboard), kanban moves, dark mode toggle.
- A manual live run is documented with its numbers before release.

## Delivery
Large; delivered as ordered milestones, each committed and pushed with CI
green:
1. Polite HTTP client, companies table, adapter-type review (ADR 0004).
2. New ATS adapters and JSON-LD adapter, closed-job tracking.
3. Discovery (Common Crawl, seeds, link following) and detection.
4. Research profiles, runs, scoring, matches, worker pipeline, API.
5. Full-text search, facets, country inference.
6. UI redesign: shell, dark mode, dashboard, research screens.
7. Job browser, kanban, companies, settings.
8. Live end-to-end run, docs, CHANGELOG, PHASES.md (Phase 9 entry).

## Left to the builder
Exact keyword and synonym lists, scoring weights within the stated caps,
seed list contents (verified), table and column names, component design,
polling intervals within reason, and exact CDX query strategy.

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

---

# Addendum 2026-10-18: Modern UI refresh and local AI enrichment

Status: Draft for approval. Added after the first scope shipped. The sections
above are unchanged. This addendum adds two independent work items; either
can ship without the other. Plan: milestones 9 and 10 in
[the implementation plan](../plans/2026-10-17-career-page-research.md).
Model decision: [ADR 0005](../../adr/0005-local-llm-enrichment.md).

The original non-goal "paid or keyed services (LLM APIs)" stays in force for
hosted services. A model that runs on the user's own machine is free, needs no
key and sends nothing away, so it is allowed as an optional add-on (the
original text already left room: "they may come later as optional add-ons").

## A. Modern UI refresh

### Intent
The first redesign made the app functional and consistent. The user now wants
it to feel modern and calm, and supplied three reference real-estate
dashboards ("Aurex Living", "Homely", "Zarss"). They share: a warm off-white
neutral palette with soft sage, amber and lavender accents; very rounded cards
(24 to 32 px) with soft shadows; a calm sidebar or pill-shaped navigation
with a clearly highlighted active item; large bold numerals with small muted
labels; mini sparklines and bar charts; list rows with a thumbnail or avatar;
a list + detail layout with filter chips; near-black primary buttons; generous
whitespace. The references are mood, not a template to copy: no layout,
asset, name or icon is taken from them.

### Goals
- A warm light theme and a matching dark theme built on semantic design
  tokens, so a screen never names a raw colour.
- Translate the references to RoleScout's domain:

  | Reference trait | RoleScout translation |
  |---|---|
  | Property photo thumbnail | Company tile: rounded square with the company initial on a colour derived from the company name (deterministic, no images fetched, no logo scraping) |
  | Price / rating numeral | Match score as a large numeral inside a progress ring (`ScoreRing`), 0 to 100 |
  | Stat cards with sparkline | Dashboard: new matches today, open jobs tracked, applications in progress, interviews upcoming; each with a 14-day sparkline where a series exists |
  | Filter chips above the list | Jobs: chips for posted window, work arrangement, seniority, sort; the facet panel stays for technologies, countries and companies |
  | Three panes | Jobs keeps list + detail (already three-column with facets); detail pane gets the large score, summary, reasons |
  | Status pills | Run state, company status, application stage |
  | Dark ink primary button | Primary action is ink on light and cream on dark; accent colours never carry primary actions |
  | Pill navigation | Segmented pill controls for in-page switches (Applications board/list, Jobs sort, Research run filters). App-level navigation remains the sidebar |
- Keep: sidebar navigation on desktop, menu sheet on mobile, system-following
  theme with manual toggle, English and Turkish for every string.
- Accessibility is a requirement, not polish: text contrast at least 4.5:1
  (3:1 for large text and UI boundaries) in both themes, checked by an
  automated test on the tokens; visible focus rings on every interactive
  element; every action reachable by keyboard; ring and sparkline carry text
  alternatives; `prefers-reduced-motion` disables transitions and ring
  animation; no information by colour alone (status pills include a text
  label).
- Works at 375 px with no horizontal scroll; touch targets at least 40 px.

### Non-goals
- No new dependency: no chart library, no component kit, no icon pack beyond
  lucide-react, no web font download (the system font stack stays; Inter is
  used when installed).
- No photos, stock images, logos fetched from company sites, or assets from
  the reference mockups.
- No change to routes, data flow or API behaviour, except one small read-only
  activity series endpoint for the dashboard sparklines.
- No animation beyond short transitions and a ring fill.

### Design decisions
- **Tokens**: CSS custom properties on `:root` and `.dark` registered with
  Tailwind 4 `@theme` so utilities such as `bg-canvas` and `text-ink-muted`
  exist. Proposed light values (builder may adjust within the contrast
  rule): canvas `#F5F1EA`, surface `#FFFDF9`, raised `#FFFFFF`, ink `#17171A`,
  ink-muted `#625D55`, line `#E6DFD3`, sage `#6F9A7D` (deep `#35604A`), amber
  `#E3A63B` (deep `#8A5A06`), lavender `#A79BE0` (deep `#4E4390`), danger
  `#B4412F`. Dark values are chosen separately, not by inversion.
- **Replacing the dark-mode override block**: today's `index.css` re-maps the
  Tailwind slate scale and many colour utilities under `.dark`. The refresh
  replaces that with semantic tokens and removes the override block once every
  screen is migrated, so there is one mechanism, not two.
- **Shape**: card radius 24 px (32 px for hero cards), controls 14 px, chips and
  pills fully round. Shadows are two soft layers; in dark mode shadows give
  way to a 1 px line.
- **Typography**: system stack; numerals use `tabular-nums` and weight 700;
  labels 12 to 13 px muted.
- **Charts**: hand-written SVG components (`Sparkline`, `ScoreRing`,
  `MiniBars`) with `role="img"` and an `aria-label` stating the values.
- **Company tile colours**: a fixed palette of 8 soft tint pairs chosen by a
  hash of the company name; the same company always gets the same tile.

### Open risks
- Warm accents on cream are easy to get below 4.5:1 (amber and sage text in
  particular). Mitigation: accents are used as fills with `ink` text, "deep"
  variants as text, and the contrast test fails the build otherwise.
- Replacing the dark override block touches every screen; a half-finished
  migration looks worse than either state. Mitigation: migrate screen by
  screen with both mechanisms present, delete the overrides in the last task.
- Large rounded cards waste space on small screens. Mitigation: radius and
  padding step down under `md`; verified at 375 px.
- The references are real-estate products. Copying their density would hurt a
  text-heavy job list. Mitigation: list rows stay compact, whitespace goes to
  the page and detail pane.

## B. Local AI enrichment

### Intent
Rule-based extraction (keyword lists, regexes, the place table) is fast and
predictable but misses things: salary in free text, remote policy phrased in
prose, technologies outside the keyword list, Turkish-language postings, and
"why does this match me" explanations. A small language model running on the
user's own computer can read job descriptions and fill those gaps for free,
offline, without sending a CV, profile or job text to anyone.

### Goals
- Extract structured fields from job descriptions: technologies, seniority,
  work arrangement and remote policy (including location limits such as "remote
  within EU"), employment type, salary range and currency, normalised location
  and country, spoken languages required, years of experience, a two-sentence
  summary (in the posting's language, with an English title-case tag line for
  search).
- Produce a short "why it matches" explanation for strong matches, grounded
  in the profile and the extracted fields.
- Help classify an unknown career page: choose which of the links already
  found on the page most likely leads to the job list. The model picks an
  index among extracted candidates; it never produces a URL.
- Run on a 16 GB Apple Silicon MacBook Air with 6 to 8 GB left for the model
  next to Postgres and Docker, for free, offline.
- Be strictly optional: off by default, `make dev` and CI work without it, and
  every feature degrades to the rules-based result.

### Non-goals
- No hosted LLM API, no API keys, no telemetry.
- Not a chat assistant, no autonomous agent loop, no tool calling, no web
  browsing by the model, no writing of cover letters or CVs.
- The model never decides whether to crawl a host (robots.txt and the review
  model remain the only gates) and never changes scores' rule-based baseline.
- No fine-tuning or training. No model download managed by RoleScout; the user
  installs the runtime and pulls the model.
- Not run inside Docker on macOS (Metal is unavailable there).

### Anti-goals (failure even if it works)
- A pipeline run, sync or page load that waits for the model.
- Model output written to the database without schema validation.
- A hallucinated technology, salary or country shown as fact.
- Job text or profile data leaving the machine.
- Treating text inside a job description as instructions (prompt injection).

### Decisions
- **Model**: Qwen3-4B-Instruct-2507, 4-bit (Q4_K_M, 2.5 GB download). Fallback
  Qwen3-1.7B, 4-bit (1.4 GB) for slower or busier machines. Reasoning in
  [ADR 0005](../../adr/0005-local-llm-enrichment.md).
- **Runtime**: Ollama on the host, default. llama.cpp `llama-server` (Metal) is
  a documented drop-in alternative. RoleScout only knows an OpenAI-compatible
  HTTP endpoint, so the runtime is the user's choice.
- **Where it runs**: a separate worker loop (`enrich`), not a research pipeline
  stage. Collection, detection and scoring finish first and never wait for it;
  the pipeline's five stages are unchanged. The enrichment loop picks up jobs
  later and the UI shows fields as they arrive.
- **What is enriched**: only jobs that are new or whose description hash
  changed, open, and, in priority order, jobs with a match score at or above
  `MATCH_MIN_SCORE`, jobs saved or applied to, then the rest. A per-tick cap
  (`LLM_BATCH`) keeps the load flat.
- **Output contract**: JSON constrained by a JSON Schema sent as
  `response_format` (grammar-constrained decoding in Ollama and llama.cpp),
  then validated again in Go. Enums are closed; free text has length limits.
- **Grounding**: every extracted technology and the salary figures must occur
  in the description text (normalised, case-insensitive). Values that do not
  are dropped and counted. This is the main defence against hallucination.
- **Merge rule**: rules-based values stay authoritative. LLM values live in
  their own table. Where the rules produced `unknown`/empty, the LLM value is
  used (an "effective" value); everything shown that came from the model
  carries an "AI" mark and the model name in a tooltip. Scoring keeps the
  rules-only baseline; the LLM-derived technologies add to the technology
  component only through the effective value and never lift a job above the
  caps in the scoring section.
- **Cache**: key `(description_sha256, model, prompt_version, schema_version)`.
  Identical descriptions across companies, reposts and restarts cost nothing.
  Changing the model or prompt version re-queues lazily, lowest priority.
- **Failure handling**: per-request timeout (`LLM_TIMEOUT`, 60 s), one retry
  on invalid JSON with a stricter instruction, then a recorded `failed` row
  that is not retried until its next backoff. Three consecutive transport
  failures open a circuit breaker for 10 minutes. Endpoint unreachable is a
  state, not an error log storm.
- **Concurrency**: one request at a time by default (`LLM_CONCURRENCY=1`).
  Generation on a laptop GPU is memory-bandwidth bound; parallel requests
  mostly add latency and memory. Configurable up to 4.
- **Prompt injection**: description text is delimited and labelled as data;
  the output schema has no field that triggers an action; output is only
  stored and displayed (escaped by React); length limits apply.
- **Privacy**: the endpoint should be loopback or `host.docker.internal`. The
  setting page warns when `LLM_BASE_URL` is not a local address. Nothing is
  sent anywhere else.
- **Docker on macOS**: Docker Desktop cannot pass the Apple GPU to
  containers, so the model server runs on the host and the worker container
  calls `http://host.docker.internal:11434/v1`. On Linux, compose maps
  `host.docker.internal` with `host-gateway`.
- **Admin UI**: Settings gets a "Local AI" card: enabled, endpoint reachable,
  model present, queue size, enriched, failed, average latency, last error,
  "Test connection" button. Read-only configuration (environment), like the
  crawl settings.

### Data model changes (summary)
`job_enrichments` (job id, description hash, model, prompt version, schema
version, status `ok|failed`, extracted JSON, error, latency, tokens, created
at; unique on the cache key), `match_explanations` (workspace id, profile id,
job id, description hash, model, text, created at; workspace-private),
`companies.classification` hints stay in the existing detection log, not a
new table.

### API (new)
- `GET /admin/llm` status and counters; `POST /admin/llm/check` runs a
  one-token request and returns reachability, model presence and latency.
- Job DTO gains `ai {summary, salary, remotePolicy, languages, yearsExperience, model}`
  (omitted when absent) and matches gain `explanation` (when present).
- Search/facets use effective seniority, arrangement, technologies, country.

### Configuration (new variables, defaults)
`LLM_ENABLED=false`, `LLM_BASE_URL=http://host.docker.internal:11434/v1`
(`http://127.0.0.1:11434/v1` for host runs), `LLM_MODEL=qwen3:4b-instruct-2507-q4_K_M`,
`LLM_FALLBACK_MODEL=qwen3:1.7b-q4_K_M` (used by the circuit breaker's
degraded mode, empty disables), `LLM_API_KEY=` (optional, for llama.cpp
`--api-key`), `LLM_TIMEOUT=60s`, `LLM_CONCURRENCY=1`, `LLM_BATCH=25` (jobs per
worker tick), `LLM_MAX_INPUT_CHARS=6000`, `LLM_MAX_OUTPUT_TOKENS=400`.

### Testing
- A fake OpenAI-compatible server (`httptest`, package `llmtest`) returns
  scripted completions: valid, invalid JSON, wrong enum, hallucinated
  technology, slow (timeout), 500, connection refused, and a prompt-injection
  echo. No test needs a model or network.
- Table tests for validation, grounding, normalisation and the merge rule;
  golden fixtures in English and Turkish.
- Worker tests: enrichment never blocks a pipeline run, caching prevents a
  second call, circuit breaker opens and closes, cancel drains cleanly.
- Web tests: Settings card states (off, unreachable, ok), AI badge rendering,
  both locales.
- Quality is measured, not assumed: a labelled set of 60 real descriptions
  (30 English, 30 Turkish; stored as fixtures with the source URLs and a
  licence-safe excerpt policy, or synthetic if a posting cannot be stored)
  gives field-level accuracy and valid-JSON rate per model. The numbers go
  into ADR 0005 and replace the estimates there.

### Open risks
- **Throughput is the main risk.** Estimates (not measurements on the user's
  machine, see ADR 0005): about 30 tokens/s generation and a couple of hundred
  tokens/s prompt processing for the 4B model on a base M2/M3 Air, so roughly
  8 to 15 s per job; a thousand jobs take hours. Mitigations: prioritised queue
  (matched jobs first), input cap, short outputs, prefix caching of the fixed
  prompt, the 1.7B fallback, and the rule that nothing waits for it. A fanless
  Air also throttles under sustained load.
- **Memory**: model 2.5 GB plus KV cache for a 4 096-token context plus
  Postgres and Docker Desktop's VM. Docker Desktop's memory setting should be
  capped (4 GB is enough for this stack, to verify).
- **Turkish quality of a 4B model** is good enough for extraction of technology
  names and numbers, less so for nuanced remote-policy wording. Only 61.6
  MMLU-ProX / 69.0 MultiIF are published, in aggregate, not for Turkish
  alone; the labelled set will tell.
- **Ollama `/v1` JSON-schema support** is documented for the native API and
  described but not explicitly confirmed for `/v1/chat/completions`. Task 24
  starts with a spike; if `/v1` ignores the schema the client uses the native
  `/api/chat` `format` field behind the same interface.
- **Model drift**: an upstream re-tag of `qwen3:4b-instruct-2507-q4_K_M`
  changes behaviour. The model name is stored per row, and the evaluation set
  is re-run on upgrade.
- **Licence**: Qwen3 is Apache-2.0 (verified on the model cards). Gemma and
  Llama have custom licences and were not chosen.

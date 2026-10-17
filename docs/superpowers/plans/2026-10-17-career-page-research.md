# Career Page Research and UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** RoleScout discovers thousands of companies' career pages for free, collects their jobs politely, scores them against per-workspace research profiles with visible pipeline stages, and presents everything in a redesigned UI.

**Architecture:** A shared polite HTTP client (`internal/crawl`) feeds new ATS and JSON-LD adapters behind the existing `ingest.Adapter` contract. A `discovery` package grows a global `companies` catalog from Common Crawl, seeds and links; a `research` package runs a five-stage pipeline in the worker, records stage state and events in PostgreSQL, and scores jobs. The API exposes profiles, runs, companies, facets and admin endpoints; the React app polls runs and gets a new shell, dashboard, research, three-column job browser, kanban and companies pages.

**Tech Stack:** Go 1.27, Fiber v3, pgx v5, PostgreSQL 17, `golang.org/x/net/html`; React 19, TanStack Query 5, React Router 7, React Hook Form + Zod, Tailwind 4, lucide-react, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-17-career-page-research-design.md`

## Global Constraints
- No new dependencies except `golang.org/x/net` promoted to direct (`go get golang.org/x/net@v0.61.0`). Web: none.
- Go tests beside code (`*_test.go`); web tests in `__tests__` folders. DB tests use `dbtest` and skip without `TEST_DATABASE_URL`.
- Every user-visible string exists in `en` and `tr` in `web/src/lib/messages.ts` with identical keys.
- Requests identify as `RoleScout/<version> (+<CRAWL_USER_AGENT_URL>)`; robots.txt is respected for every host, including vendor API hosts.
- Defaults: `CRAWL_CONCURRENCY=16`, `CRAWL_HOST_INTERVAL=1s`, `CRAWL_VENDOR_CONCURRENCY=4`, vendor interval `250ms`, `DISCOVERY_INTERVAL=168h`, `COMPANY_SYNC_INTERVAL=12h`, `RESEARCH_INTERVAL=12h`, `JSONLD_MAX_PAGES=200`, `MATCH_MIN_SCORE=40`, `CLOSED_JOB_RETENTION=2160h` (`0` = keep forever).
- Refresh tiers: companies with a match in any profile 12 h, open jobs but no match 72 h, `empty` 168 h; failure backoff exponential capped at 168 h.
- Scoring caps: title 40, technologies 30, location/arrangement 15, seniority 10, freshness 5; minimum stored score 40.
- `workday` adapter ships un-reviewed and disabled; the other adapters are reviewed by migration.
- Saved jobs and jobs with applications are never deleted.
- One milestone = one commit (or a few), pushed to `origin main`, CI green before the next milestone.
- Migrations: new files in `api/migrations`, timestamp `20261018HHMMSS_*.sql`, never edit existing ones.

## Review Focus
1. A career page URL that redirects (company.com/careers → boards.greenhouse.io/acme) must be detected as the final ATS board, not `unknown` — test in Task 6.
2. A board that starts returning 404 or errors must not close its jobs on the first failure, but its listings close after 7 days of continuous failure so dead companies do not linger — test in Task 3.
3. The same job on Greenhouse and on the company's JSON-LD page must stay one job (existing dedupe key) linked to the company — test in Task 4.
4. A worker restart or cancel in the middle of a run must leave the run `failed`/`cancelled` with a reason and release the shared stage lock — test in Task 9.
5. Malformed, huge or `@graph`-wrapped JSON-LD and non-HTML responses must be skipped with a logged reason, never crash a run — test in Task 5.

---

## Milestone 1: Polite crawling, companies, adapter review

### Task 1: Polite HTTP client

**Files:**
- Create: `api/internal/crawl/client.go`, `api/internal/crawl/robots.go`, `api/internal/crawl/limiter.go`
- Test: `api/internal/crawl/client_test.go`, `api/internal/crawl/robots_test.go`
- Modify: `api/go.mod` (x/net direct, used in Task 5), `api/internal/config/config.go` + `config_test.go`, `docs/configuration.md`, `.env.example`

**Interfaces:**
- Produces:
  - `type Options struct { UserAgent string; HostInterval time.Duration; VendorHosts []string; VendorConcurrency int; VendorInterval time.Duration; Timeout time.Duration; MaxBodyBytes int64; RobotsTTL time.Duration; Transport http.RoundTripper }`
  - `func New(o Options) *Client`
  - `func (c *Client) Get(ctx context.Context, rawURL, accept string, extra http.Header) (*Response, error)`
  - `type Response struct { StatusCode int; Header http.Header; Body []byte; FinalURL string }`
  - `var ErrDisallowed = errors.New("disallowed by robots.txt")`
  - `type StatusError struct { StatusCode int; RetryAfter time.Duration }` (returned for non-2xx except 304)
  - `func IsTemporary(err error) bool` (network errors, timeouts, 429, 5xx)
  - `var DefaultVendorHosts = []string{"boards-api.greenhouse.io","api.lever.co","api.eu.lever.co","api.ashbyhq.com","apply.workable.com","api.smartrecruiters.com"}` (subdomain vendors `*.recruitee.com`, `*.jobs.personio.de|com`, `*.bamboohr.com`, `*.myworkdayjobs.com` count as one vendor host each via suffix match)
  - Config fields: `CrawlConcurrency int`, `CrawlHostInterval time.Duration`, `CrawlVendorConcurrency int`, `CrawlUserAgentURL string`, `DiscoveryInterval`, `CompanySyncInterval`, `ResearchInterval time.Duration`, `JSONLDMaxPages int`, `MatchMinScore int`, `ClosedJobRetention time.Duration`

- [ ] **Step 1: Write failing tests** with `httptest.Server`:
  - `TestRobotsDisallowBlocksPath`: robots `User-agent: *\nDisallow: /private` → `Get(.../private/x)` returns `ErrDisallowed`, `/public` returns 200; robots fetched once for two calls.
  - `TestRobotsMissingAllowsAll`: robots 404 → allowed; robots 5xx → treat as disallow-all for that host for this TTL (`ErrDisallowed`).
  - `TestHostIntervalSpacesRequests`: 3 calls to one non-vendor host with `HostInterval=100ms` take ≥200ms; calls to two different hosts run in parallel (<100ms total extra).
  - `TestVendorHostAllowsConcurrency`: host listed in `VendorHosts`, `VendorConcurrency=4` → 4 concurrent requests observed by server.
  - `TestRetryAfterReturnedOn429`: 429 with `Retry-After: 7` → `StatusError{429, 7s}`, `IsTemporary` true; 404 → not temporary.
  - `TestBodyLimit`: body > `MaxBodyBytes` → error mentioning "larger than".
  - `TestUserAgentSent`: server sees configured User-Agent.
  - `TestRedirectToNonHTTPRejected` and `FinalURL` set after a 302.
  - `parseRobots` table test: group matching on `RoleScout` product token beats `*`, longest-match `Allow` overrides `Disallow`, `$` and `*` wildcards.
  - Config: defaults equal Global Constraints; `CRAWL_CONCURRENCY=0` and `MATCH_MIN_SCORE=101` rejected.
- [ ] **Step 2:** `cd api && go test ./internal/crawl/ ./internal/config/` → FAIL (package missing).
- [ ] **Step 3: Implement.** Conditional requests (spec "Polite crawling"): `Get` sends `If-None-Match`/`If-Modified-Since` from a small in-memory validator cache (URL → ETag/Last-Modified, LRU 10 000) only when the caller passes `extra` with `X-RoleScout-Conditional: 1`; used for robots.txt, Common Crawl `collinfo.json` and JSON-LD job pages. Board snapshot APIs always fetch in full because closing jobs needs the whole snapshot (builder's call, recorded in ADR 0004). A 304 returns `Response{StatusCode: 304}` without error. Test: `TestConditionalRequest304`. Limiter: per-host `chan struct{}` semaphore (size 1, or `VendorConcurrency` for vendor hosts) plus last-request timestamp per slot; robots cache `map[host]robotsEntry` under mutex with TTL 24h; robots fetched with the same client but bypassing the robots check. `MaxBodyBytes` default 32 MiB, `Timeout` 60s.
- [ ] **Step 4:** tests pass with `-race`.
- [ ] **Step 5:** add variables to `docs/configuration.md` (new "Crawling and research" table) and `.env.example`.

### Task 2: Companies, adapter-type review, admin flag (ADR 0004)

**Files:**
- Create: `api/migrations/20261018100000_add_companies_and_adapter_reviews.sql`, `api/internal/infrastructure/database/companies.go`, `docs/adr/0004-discovery-and-adapter-review.md`
- Modify: `api/internal/domain/domain.go`, `api/internal/infrastructure/database/sources.go`, `auth.go`, `users.go`, `api/cmd/admin/main.go`, `docs/sources/README.md`, `docs/adr/0003-source-ingestion.md` (status line: "review section superseded by 0004")
- Test: `api/internal/infrastructure/database/companies_test.go`, `sources_test.go`, `auth_test.go`

**Interfaces:**
- Produces (domain):
  - `type CompanyStatus string` = `pending|active|empty|unreachable|blocked_by_robots|disabled`
  - `type Company struct { ID, Name string; Domain *string; CareersURL string; ATS string; ATSID *string; Origin string; Status CompanyStatus; LastCheckedAt *time.Time; LastError *string; FailureCount int; NextCheckAt time.Time; SourceID *string; OpenJobs int; CreatedAt, UpdatedAt time.Time }` — `Origin` ∈ `commoncrawl|seed|link|manual`
  - `type AdapterReview struct { Adapter string; Enabled bool; ReviewedAt *time.Time; Note *string }`
  - `User.IsAdmin bool`
- Produces (database):
  - `NewCompanyRepository(pool) *CompanyRepository`
  - `UpsertDiscovered(ctx, p CompanyParams) (domain.Company, bool /*created*/, error)` — conflict on `(ats, ats_id)` when set, else on `careers_url`; never downgrades `origin=manual`
  - `type CompanyParams struct { Name, CareersURL, ATS, Origin string; ATSID, Domain *string }`
  - `PendingDetection(ctx, limit int) ([]domain.Company, error)` (status `pending` or `ats='unknown'` with `next_check_at <= now()`)
  - `RecordDetection(ctx, id string, ats string, atsID *string, status domain.CompanyStatus, errMsg *string) (domain.Company, error)`
  - `AttachSource(ctx, companyID string, p SourceParams) (domain.JobSource, error)` — creates/updates the company's `job_sources` row, key `company-<ats>-<sanitized id>`, `enabled=true`
  - `RecordSyncOutcome(ctx, companyID string, run domain.IngestionRun) error` — sets status `active|empty|unreachable`, failure count, `next_check_at` backoff
  - `List(ctx, f CompanyFilter) ([]domain.Company, bool, error)`; `type CompanyFilter struct { Query, ATS string; Status domain.CompanyStatus; WorkspaceID string /*marks blocked*/; AfterName, AfterID string; Limit int }`
  - `SetDisabled(ctx, id string, disabled bool) (domain.Company, error)`
  - `Block(ctx, workspaceID, companyID string) error`, `Unblock(...)`, `CountsByATS(ctx) (map[string]int, error)`
  - `AdapterReviews(ctx) ([]domain.AdapterReview, error)`, `ReviewAdapter(ctx, adapter, note string, enabled bool) (domain.AdapterReview, error)`
- Schema decisions:
  - `companies(id uuid pk, name text, domain text, careers_url text unique check '^https?://', ats text default 'unknown', ats_id text, origin text, status text, last_checked_at, last_error, failure_count int default 0, next_check_at timestamptz default now(), created_at, updated_at)`, `UNIQUE (ats, ats_id)`, index `(status, next_check_at)`.
  - `job_sources.company_id uuid unique references companies on delete cascade`.
  - `source_adapter_reviews(adapter text pk, enabled bool, reviewed_at, note)`; rows inserted for `greenhouse, lever, ashby, workable, smartrecruiters, recruitee, personio, bamboohr, jsonld` with notes "Public job board API intended for publishing a company's jobs; terms checked 2026-10-18; see docs/sources/<adapter>.md" (`jsonld`: "Public JobPosting structured data; robots.txt respected"); `workday` row with `enabled=false`, no review.
  - Drop CHECK `job_sources_enabled_requires_review`; add BEFORE INSERT OR UPDATE trigger raising `check_violation` with `CONSTRAINT = 'job_sources_enabled_requires_review'` unless `NOT NEW.enabled OR (own review present) OR (adapter review enabled)` — existing error mapping to `ErrSourceNotReviewed` keeps working.
  - `workspace_company_blocks(workspace_id, company_id, created_at, pk both)`.
  - `users.is_admin bool default false`; migration sets the earliest user admin; `Register` sets `is_admin = NOT EXISTS (SELECT 1 FROM users)` inside its transaction.

- [ ] **Step 1: Failing DB tests:**
  - `TestSourceEnableNeedsAdapterOrSourceReview`: source with adapter `workday` → `SetEnabled` returns `ErrSourceNotReviewed`; after `ReviewAdapter("workday", "note", true)` it succeeds; adapter `greenhouse` enables without own review.
  - `TestUpsertDiscoveredDedupesByBoard`: same `(greenhouse, acme)` twice → one row, `created` false second time; manual origin kept.
  - `TestAttachSourceIsIdempotent`: second call updates config, same source ID.
  - `TestRecordSyncOutcomeBackoff`: failed run ×3 → `failure_count=3`, `next_check_at ≈ now+4h` (1h·2^(n-1)), capped at 168h; succeeded with 0 fetched → `empty`.
  - `TestBlocksAreWorkspacePrivate`: block in W1 → `List(WorkspaceID=W2)` not marked blocked.
  - `TestFirstUserIsAdmin` (auth): first registered admin, second not.
- [ ] **Step 2:** run `go test ./internal/infrastructure/database/` with `TEST_DATABASE_URL` → FAIL.
- [ ] **Step 3:** migration + repositories; `domain.User.IsAdmin` scanned everywhere users are scanned.
- [ ] **Step 4:** admin CLI: `admin adapters list`, `admin adapters review -adapter NAME -note TEXT`, `admin adapters enable|disable -adapter NAME`; `admin companies list [-status S]`, `admin companies add -url URL [-name N]` (inserts `pending`, origin `manual`). Update `usage`.
- [ ] **Step 5:** ADR 0004 records: adapter-type review, discovery sources, polite crawling limits, sharing model, retention. Tests pass; commit "Add polite crawler, company catalog and adapter-type review"; push; CI green.

## Milestone 2: Adapters and closed jobs

### Task 3: Ingestion changes — company link, hints, closed jobs, parallel sync

**Files:**
- Create: `api/migrations/20261018110000_add_job_closing_and_country.sql`
- Modify: `api/internal/ingest/adapter.go`, `normalize.go`, `runner.go`, `api/internal/infrastructure/database/jobs.go`, `api/internal/domain/domain.go`, `api/internal/ingest/greenhouse/greenhouse.go`, `api/internal/ingest/adapters/adapters.go`, `api/cmd/worker/main.go`, `api/cmd/admin/main.go`
- Test: `api/internal/ingest/runner_test.go`, `normalize_test.go`, `integration_test.go`, `database/jobs_test.go`, `greenhouse/greenhouse_test.go`

**Interfaces:**
- Produces:
  - `RawListing` gains `CountryCode string` (ISO 3166-1 alpha-2 when the source states it) and `Arrangement string` (`remote|hybrid|onsite` when the source states it; overrides inference).
  - `database.IngestedListing` gains `CompanyID *string`, `CountryCode *string`.
  - `domain.Job` gains `CompanyID *string`, `CountryCode *string`, `ClosedAt *time.Time`.
  - `func (r *JobRepository) CloseMissing(ctx context.Context, sourceID string, seenExternalIDs []string, at time.Time) (int64, error)` — sets `job_source_listings.closed_at=at` for listings of the source not in the list; sets `jobs.closed_at=at` for jobs whose listings are all closed. `UpsertListing` clears both `closed_at`s.
  - `func (r *JobRepository) CloseFailedSources(ctx context.Context, failingSince time.Duration) (int64, error)` — closes listings of sources whose latest successful run is older than `failingSince` (7×24h) while they have failed runs since.
  - `func (r *Runner) RunDue(ctx context.Context, o RunDueOptions) error`; `type RunDueOptions struct { Concurrency int; OnResult func(domain.JobSource, domain.IngestionRun, error) }` — calls `defaults()` once before starting goroutines; worker loop passes `Concurrency: cfg.CrawlConcurrency`.
  - `Runner` gains `Companies interface{ RecordSyncOutcome(ctx, companyID string, run domain.IngestionRun) error }` (nil-safe) called after every sync of a source with `CompanyID`.
  - `domain.JobSource` gains `CompanyID *string`.
  - `func adapters.Registry(c *crawl.Client) ingest.Registry`; greenhouse `New(c *crawl.Client)` uses the crawl client (no own `http.Client`).
  - `func ingest.InferCountry(location string) string` backed by embedded `api/internal/ingest/places.tsv` (`name<TAB>ISO2`, English and Turkish country names, ISO codes, ~300 major cities; matched as whole words, last match wins: "Berlin, Germany" → DE).
- Migration: `job_source_listings.closed_at`, `jobs.closed_at`, `jobs.company_id references companies on delete set null`, `jobs.country_code char(2)`; indexes `jobs (closed_at) WHERE closed_at IS NULL` is not useful — use partial `jobs_open_first_seen_idx ON jobs (first_seen_at DESC, id DESC) WHERE closed_at IS NULL`.

- [ ] **Step 1: Failing tests:**
  - `TestSyncClosesMissingListings` (integration): snapshot {A,B} then {A} → B's job `closed_at` set; snapshot {A,B} again → reopened (`closed_at` nil).
  - `TestPartialSkipDoesNotCloseSkipped`: listing in snapshot that fails normalization is passed as seen → not closed.
  - `TestFailedFetchDoesNotClose`: fetch error → no closing.
  - `TestCloseFailedSourcesAfterSevenDays` (Review Focus 2): last success 8 days ago and failures since → closed; 6 days → open.
  - `TestRunDueParallel`: 10 fake sources with 50ms fetches, `Concurrency: 5` → total < 200ms; `OnResult` called 10 times; `-race` clean.
  - `TestArrangementHintOverridesInference`, `TestInferCountry` table: "Istanbul, Türkiye"→TR, "İstanbul"→TR, "Remote - Germany"→DE, "London, UK"→GB, "Remote"→"" , "Georgia, US"→US (last match).
  - `TestRecordSyncOutcomeCalledForCompanySources`.
- [ ] **Step 2:** FAIL.
- [ ] **Step 3:** implement; runner after a successful fetch calls `CloseMissing(source.ID, seenIDs, seenAt)`; worker additionally calls `CloseFailedSources(ctx, 7*24h)` once per tick.
- [ ] **Step 4:** pass; existing tests updated for the new Registry signature.

### Task 4: ATS adapters

**Files:**
- Create one package each under `api/internal/ingest/`: `lever`, `ashby`, `workable`, `smartrecruiters`, `recruitee`, `personio`, `bamboohr`, `workday`; each `<name>.go`, `<name>_test.go`, `testdata/*.json|xml` fixtures; `docs/sources/<name>.md` each.
- Modify: `api/internal/ingest/adapters/adapters.go`, `docs/sources/README.md`

**Interfaces:**
- Consumes: `crawl.Client.Get`, `crawl.IsTemporary`, `ingest.Retryable`, `ingest.StringConfig`.
- Produces: each package `func New(c *crawl.Client) *Adapter` implementing `ingest.Adapter`; `BaseURL string` field overridable in tests. Config (all accept optional `"company"` display name; unknown fields rejected):
  - `lever`: `{"site": "acme", "region": "eu"?}` → `GET https://api.lever.co/v0/postings/{site}?mode=json` (`api.eu.lever.co` for eu); `workplaceType` → Arrangement; `country` → CountryCode.
  - `ashby`: `{"board": "acme"}` → `GET https://api.ashbyhq.com/posting-api/job-board/{board}?includeCompensation=true`; `isRemote`/`workplaceType`; `address.postalAddress.addressCountry`.
  - `workable`: `{"account": "acme"}` → `GET https://apply.workable.com/api/v1/widget/accounts/{account}?details=true`; `telecommuting` → remote; `country_code`.
  - `smartrecruiters`: `{"company": ...}` conflicts with display name → use `{"company_id": "Acme1"}`; `GET https://api.smartrecruiters.com/v1/companies/{id}/postings?limit=100&offset=N` until `offset >= totalFound` (max 50 pages); descriptions fetched per posting only when `jobAd` absent — skip, use posting `jobAd.sections` when present, else empty description.
  - `recruitee`: `{"subdomain": "acme"}` → `GET https://{subdomain}.recruitee.com/api/offers/`; `remote`, `country_code`.
  - `personio`: `{"subdomain": "acme", "tld": "de"|"com"}` → `GET https://{subdomain}.jobs.personio.{tld}/xml?language=en`; XML via `encoding/xml`.
  - `bamboohr`: `{"subdomain": "acme"}` → `GET https://{subdomain}.bamboohr.com/careers/list` (JSON `result[]`), detail URL `https://{subdomain}.bamboohr.com/careers/{id}`; description empty (list has none) — acceptable.
  - `workday`: `{"host": "acme.wd5.myworkdayjobs.com", "tenant": "acme", "site": "External"}` → `POST https://{host}/wday/cxs/{tenant}/{site}/jobs` with `{"limit":20,"offset":N,"searchText":""}` paged (max 100 pages). Needs a `crawl.Client.Post(ctx, url, contentType string, body []byte) (*Response, error)` — add to Task 1's client in this task with the same robots/limit rules.
  - Exact JSON fields: take from a real response saved as fixture at build time (fetch once manually with curl, trim to 2–3 jobs, strip nothing else); verify each endpoint's documentation/terms and write them in `docs/sources/<name>.md` with the date.
- [ ] **Step 1:** for each adapter, failing tests: `TestValidate` (missing/invalid/unknown fields), `TestFetchMapsFixture` (ExternalID, URL absolute, Title, Company, Location, CountryCode, Arrangement, PostedAt), `TestFetchNotFound` (404 → non-retryable error "board not found"), `TestFetchServerErrorRetryable` (503 → `ingest.IsRetryable`), paging test for smartrecruiters and workday.
- [ ] **Step 2:** FAIL; **Step 3:** implement; **Step 4:** PASS.
- [ ] **Step 5:** `TestDedupeAcrossGreenhouseAndJSONLD` (Review Focus 3) lives in `ingest/integration_test.go` after Task 5: same company/title/location from two sources → one job, two listings, `company_id` set.

### Task 5: JSON-LD adapter

**Files:**
- Create: `api/internal/ingest/jsonld/jsonld.go`, `extract.go`, `jsonld_test.go`, `testdata/*.html`; `docs/sources/jsonld.md`
- Modify: `adapters.go`

**Interfaces:**
- Produces:
  - `func New(c *crawl.Client, maxPages int) *Adapter`; config `{"url": "https://acme.com/careers", "company"?: "Acme"}`.
  - `func ExtractJobPostings(body []byte, pageURL string) ([]Posting, []string /*same-site links under the careers path*/, error)` in `extract.go` (also used by detection in Task 6).
  - `type Posting struct { ID, URL, Title, Company, Location, CountryCode, Arrangement, Description, EmploymentType string; PostedAt *time.Time }`
- Rules: parse with `golang.org/x/net/html`; read every `<script type="application/ld+json">`; accept an object, an array, or `@graph`; `@type` may be a string or array containing `JobPosting`; `jobLocationType: TELECOMMUTE` → remote; `identifier.value` or `url` or page URL + title hash as ID; follow links that share the careers URL's host and path prefix, breadth-first, up to `maxPages` pages, skip non-`text/html` responses; ignore postings whose `validThrough` is in the past.
- [ ] **Step 1:** failing tests: single object, array, `@graph`, `@type` array, malformed JSON in one script while another is valid (valid one returned, no error), non-HTML response skipped, page limit honored (fixture site with 5 linked pages, `maxPages=3` → 3 fetched), expired `validThrough` dropped, HTML entities in description converted by existing `HTMLToText`. (Review Focus 5.)
- [ ] **Step 2–4:** FAIL → implement → PASS.
- [ ] **Step 5:** commit Milestone 2 "Add ATS and JSON-LD adapters and close jobs that disappear"; push; CI green.

## Milestone 3: Discovery and detection

### Task 6: Board URL parsing and career page detection

**Files:**
- Create: `api/internal/discovery/boards.go`, `detect.go`, `boards_test.go`, `detect_test.go`, `testdata/*.html`

**Interfaces:**
- Produces:
  - `type Board struct { ATS, ID string; Config map[string]any }`
  - `func ParseBoardURL(raw string) (Board, bool)` — recognizes `boards.greenhouse.io/{t}`, `job-boards.greenhouse.io/{t}`, `boards.greenhouse.io/embed/job_board?for={t}`, `jobs.lever.co/{s}`, `jobs.eu.lever.co/{s}`, `jobs.ashbyhq.com/{b}`, `apply.workable.com/{a}`, `jobs.smartrecruiters.com/{c}`, `{s}.recruitee.com`, `{s}.jobs.personio.de|com`, `{s}.bamboohr.com/careers|jobs`, `{tenant}.wd{n}.myworkdayjobs.com/{locale?}/{site}`; rejects reserved path segments (`embed`, `api`, `static`, `assets`, `v0`, `careers` for Lever…) and IDs not matching `^[A-Za-z0-9_.-]{1,100}$`. `Config` is the adapter config for Task 4/5 adapters.
  - `func (b Board) SourceKey() string` → `company-<ats>-<lowercased id with [^a-z0-9_-] → ->`.
  - `type Detector struct { Client *crawl.Client }`
  - `func (d *Detector) Detect(ctx context.Context, careersURL string) (Detection, error)`; `type Detection struct { Board Board /*ATS "unknown" when none*/; FinalURL string; Name string /*og:site_name or <title>*/; Links []Board /*other boards linked from the page*/ }`
- Detection order: `ParseBoardURL(careersURL)`; else GET (redirects followed) and `ParseBoardURL(FinalURL)`; else scan `<script src>`, `<iframe src>`, `<a href>` with `ParseBoardURL`, plus Greenhouse embed `grnhse_board_token`/`for=` patterns; first board wins, all distinct boards go to `Links`; else `jsonld.ExtractJobPostings` non-empty → `Board{ATS:"jsonld", ID: FinalURL, Config: {"url": FinalURL}}`; else `unknown`. `ErrDisallowed` → returned so the caller records `blocked_by_robots`.
- [ ] **Step 1:** failing tests: `ParseBoardURL` table (≥ 30 rows incl. reserved segments and query-string embed), `TestDetectFollowsRedirectToATS` (Review Focus 1: server `/careers` 302 → fake greenhouse board URL host via test transport mapping), `TestDetectEmbedScript`, `TestDetectIframeLever`, `TestDetectJSONLD`, `TestDetectUnknown`, `TestDetectRobotsBlocked`.
- [ ] **Step 2–4:** FAIL → implement → PASS. Test transport: `crawl.Options.Transport` rewrites real ATS hosts to the `httptest` server.

### Task 7: Common Crawl discovery and seeds

**Files:**
- Create: `api/internal/discovery/commoncrawl.go`, `commoncrawl_test.go`, `seeds.go`, `seeds_test.go`, `seeds/companies.txt`, `testdata/collinfo.json`, `testdata/cdx-*.jsonl`
- Modify: `api/cmd/admin/main.go` (`admin seeds check`)

**Interfaces:**
- Produces:
  - `type CommonCrawl struct { Client *crawl.Client; BaseURL string /*https://index.commoncrawl.org*/; PageDelay time.Duration /*default 2s*/; MaxPages int /*per pattern, default 50*/ }`
  - `func (cc *CommonCrawl) LatestIndex(ctx) (string, error)` — first entry of `/collinfo.json` (`id`, e.g. `CC-MAIN-2026-38`)
  - `func (cc *CommonCrawl) Boards(ctx context.Context, index string, fn func(Board) error) error` — for each pattern in `BoardPatterns`, `GET /{index}-index?url={pattern}&output=json&fl=url&filter=status:200&showNumPages=true` then pages `&page=N`; parse JSON lines; `ParseBoardURL` each URL; calls `fn` once per distinct board.
  - `var BoardPatterns = []string{"boards.greenhouse.io/*","job-boards.greenhouse.io/*","jobs.lever.co/*","jobs.eu.lever.co/*","jobs.ashbyhq.com/*","apply.workable.com/*","jobs.smartrecruiters.com/*","*.recruitee.com","*.jobs.personio.de","*.jobs.personio.com","*.bamboohr.com/careers*","*.myworkdayjobs.com/*"}`
  - `type Seed struct { URL, Name string }`; `func Seeds() ([]Seed, error)` from embedded `seeds/companies.txt` (`URL [| Name]`, `#` comments, blank lines).
  - Discovery state: `discovery_state(key text pk, value text, updated_at)` table (add to Task 8's migration) storing `last_index` and `last_full_discovery_at` so a later run skips indexes already read.
- Seed list content: ≥ 300 career page URLs of companies that hire software engineers, including ≥ 40 companies with offices in Türkiye (e.g. Trendyol, Getir, Hepsiburada, Insider, Peak Games, Papara, Param, Iyzico, Picus, Commencis, Dream Games, Midas, Obilet, Colendi …); each verified with `admin seeds check` (prints `OK <ats> <url>` or `DEAD <url> <reason>`); dead ones removed before commit.
- [ ] **Step 1:** failing tests: `TestLatestIndex`, `TestBoardsPagesAndDedupes` (2 patterns × 2 pages fixtures with duplicates → distinct boards, page delay honored via injectable sleep), `TestBoardsStopsAtMaxPages`, `TestSeedsParse` (comments, names, invalid URL → error naming the line), `TestEmbeddedSeedsValid` (all parse, no duplicates).
- [ ] **Step 2–4:** FAIL → implement → PASS.
- [ ] **Step 5:** run `admin seeds check` against the real internet once; prune; commit Milestone 3 "Discover companies from Common Crawl, seeds and career pages"; push; CI green.

## Milestone 4: Research pipeline

### Task 8: Research schema and repositories

**Files:**
- Create: `api/migrations/20261018120000_create_research.sql`, `api/internal/infrastructure/database/research.go`, `research_test.go`
- Modify: `api/internal/domain/domain.go`, `auth.go` / `workspaces.go` (create default profile with every new workspace, in the same transaction)

**Interfaces:**
- Produces (domain):
  - `type ResearchProfile struct { ID, WorkspaceID, Name string; TitleKeywords, Keywords, Technologies, Countries, Locations, ExcludedKeywords []string; Remote RemotePreference; Seniority []string; ScheduleInterval time.Duration; LastRunAt *time.Time; CreatedAt, UpdatedAt time.Time }`; `type RemotePreference string` = `any|remote_only|remote_or_hybrid|onsite_ok`
  - `type RunState string` = `queued|running|succeeded|failed|cancelled`; `type Stage string` = `discovery|detection|collection|scoring|result`
  - `type StageProgress struct { State string /*pending|running|done|skipped|failed*/; Total, Done, Failed int; Detail map[string]int; StartedAt, FinishedAt *time.Time; Note string }`
  - `type ResearchRun struct { ID, WorkspaceID, ProfileID string; State RunState; Stage Stage; Stages map[Stage]StageProgress; Trigger string /*manual|schedule*/; Error *string; CancelRequested bool; QueuedAt time.Time; StartedAt, FinishedAt *time.Time; Summary map[string]int }`
  - `type RunEvent struct { ID int64; RunID string; At time.Time; Level string /*info|warn|error*/; Stage Stage; Company string; Message string }`
  - `type MatchReason struct { Code string; Params map[string]string }`
  - `type Match struct { ProfileID, JobID string; Score int; Reasons []MatchReason; New bool; FirstMatchedAt time.Time }`
  - Default profile values: name "Full Stack Developer", TitleKeywords `["full stack","fullstack","full-stack"]`, Technologies `["react","typescript","node.js","javascript","postgresql"]`, Remote `any`, Seniority `[]` (any), ScheduleInterval 12h.
- Produces (database), `NewResearchRepository(pool)`:
  - Profiles: `CreateProfile(ctx, workspaceID string, p ProfileParams) (domain.ResearchProfile, error)`, `GetProfile(ctx, workspaceID, id)`, `ListProfiles(ctx, workspaceID)`, `UpdateProfile(ctx, workspaceID, id string, p ProfileParams)`, `DeleteProfile(ctx, workspaceID, id)`; `type ProfileParams` mirrors the editable fields.
  - Runs: `EnqueueRun(ctx, workspaceID, profileID, trigger string) (domain.ResearchRun, error)` — `ErrConflict` when the profile already has a queued/running run (partial unique index); `ClaimRun(ctx) (domain.ResearchRun, bool, error)` (`FOR UPDATE SKIP LOCKED`, oldest queued → running); `SaveProgress(ctx, run domain.ResearchRun) error`; `FinishRun(ctx, id string, state domain.RunState, errMsg *string, summary map[string]int) error`; `RequestCancel(ctx, workspaceID, id string) error`; `CancelRequested(ctx, id string) (bool, error)`; `GetRun(ctx, workspaceID, id)`; `ListRuns(ctx, workspaceID string, profileID string, limit int)`; `FailStaleRuns(ctx, maxAge time.Duration) (int64, error)` (running with `updated_at` older than maxAge → failed "worker stopped"); `DueProfiles(ctx, now time.Time) ([]domain.ResearchProfile, error)`.
  - Events: `AddEvents(ctx, events []domain.RunEvent) error` (batch insert; then delete beyond the newest 2 000 per run); `ListEvents(ctx, workspaceID, runID string, afterID int64, level string, limit int) ([]domain.RunEvent, error)`.
  - Matches: `ReplaceMatches(ctx, profileID string, ms []domain.Match, since *time.Time) (newCount int, err error)` — upsert, keep `first_matched_at`, `new = job.first_seen_at > since`, delete matches no longer present; `MatchesByJob(ctx, profileID string, jobIDs []string) (map[string]domain.Match, error)`.
  - Retention: `PurgeClosedJobs(ctx, olderThan time.Duration) (int64, error)` (skip saved / applications), `PurgeRunEvents(ctx, olderThan time.Duration) (int64, error)`.
- Schema: `research_profiles`, `research_runs` (stages jsonb, summary jsonb, `updated_at` for staleness), `research_run_events (id bigserial)`, `research_matches (profile_id, job_id pk; score smallint check 0..100; reasons jsonb)`, `discovery_state`; all workspace-scoped tables use composite FKs like existing ones; migration inserts the default profile for every existing workspace.
- [ ] **Step 1:** failing DB tests: profile CRUD isolation (other workspace → `ErrNotFound`), `TestEnqueueOnePerProfile`, `TestClaimRunSkipLocked` (two concurrent claims → different runs or one false), `TestFailStaleRuns`, `TestEventsCappedAt2000`, `TestReplaceMatchesNewFlagAndRemoval`, `TestPurgeClosedJobsKeepsSavedAndApplied`, `TestNewWorkspaceGetsDefaultProfile`.
- [ ] **Step 2–4:** FAIL → implement → PASS.

### Task 9: Scoring and pipeline

**Files:**
- Create: `api/internal/research/score.go`, `score_test.go`, `pipeline.go`, `reporter.go`, `pipeline_test.go`, `loop.go`
- Modify: `api/cmd/worker/main.go`, `api/internal/infrastructure/database/jobs.go` (`OpenJobsForScoring`), `companies.go` (`UpdateTiers`)

**Interfaces:**
- Consumes: Tasks 2–8.
- Produces:
  - `func Score(p domain.ResearchProfile, j domain.Job, now time.Time) (score int, reasons []domain.MatchReason, ok bool)` — components per Global Constraints; title synonyms table in `score.go` (`"full stack"` ⇢ `full stack|full-stack|fullstack|full stack engineer|full stack developer|software engineer, full stack|product engineer`); technologies: `30 * matched/len(p.Technologies)`, profile without technologies → 15 flat; location: remote job + remote-accepting profile 15, country in `p.Countries` 15, hybrid with `remote_or_hybrid` 10, onsite outside countries with `remote_only` → `ok=false`; seniority in list or list empty 10, job `unknown` 5; freshness: first seen ≤ 7 days 5. Hard filters: excluded keyword in title/description, `remote_only` + onsite. Reason codes: `title`, `tech`, `remote`, `country`, `hybrid`, `seniority`, `fresh` with params (`matched`, `total`, `value`).
  - `type Pipeline struct { Runs RunStore; Companies CompanyStore; Jobs JobStore; Discovery Discoverer; Detector *discovery.Detector; Ingest *ingest.Runner; Locker StageLocker; Config Config; Logger *slog.Logger; Now func() time.Time }` with small interfaces declared in `pipeline.go` matching Task 2/3/7/8 methods.
  - `type Config struct { DiscoveryInterval, CompanySyncInterval time.Duration; Concurrency, MatchMinScore int }`
  - `func (p *Pipeline) Execute(ctx context.Context, run domain.ResearchRun) error`
  - `type StageLocker interface { Acquire(ctx context.Context) (release func(), err error) }` — Postgres `pg_advisory_lock` on a dedicated pooled connection, key `hashtext('rolescout.stages')`; waiting logs an event "waiting for another research run".
  - `type Reporter` buffers events and progress, flushes every 1s and at stage boundaries, checks `CancelRequested` on each flush; cancel → context cancelled → run `cancelled`.
  - `func (p *Pipeline) Loop(ctx context.Context, tick time.Duration)` — each tick: `FailStaleRuns(15m)`, enqueue `DueProfiles` (trigger `schedule`), claim and execute runs one at a time, once a day `PurgeClosedJobs(ClosedJobRetention)` (skipped when 0) and `PurgeRunEvents(30 days)`; heartbeat updates `updated_at` every flush.
  - `func (r *JobRepository) OpenJobsForScoring(ctx, workspaceID string, fn func(domain.Job) error) error` — streams open jobs not from companies blocked in that workspace.
  - `func (r *CompanyRepository) UpdateTiers(ctx, top, mid, low time.Duration) error` — one SQL statement setting `job_sources.sync_interval` per tier.
- Stage behaviour:
  1. discovery: if `last_full_discovery_at` within `DiscoveryInterval` → `skipped` with note "catalog is fresh, N companies"; else seeds (`origin seed`) + Common Crawl boards of indexes not yet read (`origin commoncrawl`); `Detail` counts per ATS; `Total` = companies known after.
  2. detection: `PendingDetection` in batches of 200 with `Concurrency` workers; known-board URLs need no request; boards → `AttachSource`; `Links` → `UpsertDiscovered(origin link)`; failures counted with event per company (`warn`), `blocked_by_robots` status on `ErrDisallowed`.
  3. collection: `Ingest.RunDue(ctx, RunDueOptions{Concurrency, OnResult})`; `OnResult` increments Done/Failed and emits `info` "<company> · <ats> · N jobs" or `error` with the run error.
  4. scoring: stream jobs, `Score`, keep `score >= MatchMinScore`, `ReplaceMatches(since = profile.LastRunAt)`; `UpdateTiers(12h, 72h, 168h)`.
  5. result: `Summary` = `companies_known, companies_checked, companies_failed, jobs_collected, jobs_open, matches, new_matches`.
  Stages 1–3 run under `StageLocker`; stage 4 runs outside it.
- [ ] **Step 1:** failing tests:
  - `score_test.go` table: "Senior Full Stack Engineer" remote React/Node job vs default profile ≥ 85 with reasons `title`, `tech`, `remote`; "Accountant" → < 40; excluded keyword → `ok=false`; profile with no technologies → tech 15; `remote_only` + onsite Berlin → `ok=false`; score never > 100.
  - `pipeline_test.go` with fakes: stage order and states, `skipped` discovery when fresh, `OnResult` counts, cancel during collection → run `cancelled` and lock released (Review Focus 4), panic in a fake adapter → that company failed, run succeeded, stale run recovery via `FailStaleRuns` after simulated restart.
  - Integration (`research_integration_test.go`, DB + `httptest` fake Common Crawl/Greenhouse/Lever): a full run produces companies, jobs, matches, events, summary numbers that match fixtures.
- [ ] **Step 2–4:** FAIL → implement → PASS (`-race`).
- [ ] **Step 5:** worker runs `Runner.Loop` (scheduled syncs, parallel) and `Pipeline.Loop` (runs) concurrently; worker detects pending manual companies in each tick (detection only, outside runs).

### Task 10: Research, company and admin API

**Files:**
- Create: `api/internal/transport/http/research.go`, `companies.go`, `admin.go`, `research_test.go`, `companies_test.go`, `admin_test.go`
- Modify: `routes.go`, `dto.go`, `auth.go` (`me` returns `isAdmin`), `api/cmd/api/main.go`, `docs/api/openapi.yaml`, `docs/api/README.md`

**Interfaces:**
- Routes (all JSON, existing error format, `requireWorkspace` for `/workspaces/:workspaceID/...`, `requireAdmin` → 403 `forbidden` for `/admin/...`):
  - `GET|POST /workspaces/:ws/research-profiles`, `GET|PATCH|DELETE /workspaces/:ws/research-profiles/:profileID`
  - `POST /workspaces/:ws/research-profiles/:profileID/runs` → 202 run; 409 `conflict` if one is active
  - `GET /workspaces/:ws/research-runs?profile=&limit=`, `GET /workspaces/:ws/research-runs/:runID`, `GET /workspaces/:ws/research-runs/:runID/events?after=&level=&limit=` (limit ≤ 500), `POST /workspaces/:ws/research-runs/:runID/cancel` → 202
  - `GET /companies?q=&ats=&status=&cursor=&limit=` (workspace context via `?workspace=` to mark blocked; membership checked), `POST /companies {careersUrl, name?}` → 202 company `pending` (URL must be absolute http(s), ≤ 2 000 chars)
  - `PUT|DELETE /workspaces/:ws/company-blocks/:companyID`
  - `GET /admin/adapters`, `PATCH /admin/adapters/:adapter {enabled, note}` (note required when enabling), `PATCH /admin/companies/:companyID {disabled}`, `GET /admin/settings` (crawl config values, read-only)
- DTOs: `profileDTO`, `runDTO` (stages map with camelCase fields), `runEventDTO`, `companyDTO` (`blocked bool` when workspace given), `adapterReviewDTO`; validation: names ≤ 100 chars, each keyword ≤ 60 chars, ≤ 30 items per list, countries are ISO-2 upper-case, technologies lower-cased.
- [ ] **Step 1:** failing handler tests with fakes (existing pattern in `handlers_test.go`): profile CRUD + validation errors, other workspace → 404, run start 202 / 409, events `after` cursor, cancel, admin routes 403 for non-admin and 200 for admin, company add invalid URL → 422.
- [ ] **Step 2–4:** FAIL → implement → PASS; OpenAPI updated; commit Milestone 4 "Add research profiles, pipeline and API"; push; CI green.

## Milestone 5: Search and facets

### Task 11: Full-text search, workspace job browsing, facets

**Files:**
- Create: `api/migrations/20261018130000_add_job_full_text_search.sql`
- Modify: `database/jobs.go`, `jobs_test.go`, `transport/http/jobs.go`, `dto.go`, `routes.go`, `handlers_test.go`, `docs/api/openapi.yaml`

**Interfaces:**
- Migration: `jobs.search tsvector` maintained by a BEFORE INSERT/UPDATE trigger (title weight A; company name + technologies B; first 20 000 chars of description C; config `simple`) — trigger, not a generated column, because `array_to_string` is not immutable; backfill existing rows; GIN index.
- `JobFilter` gains: `WorkspaceID string`, `ProfileID string`, `PostedWithin time.Duration`, `Countries []string`, `CompanyIDs []string`, `IncludeClosed bool`, `Sort string` (`recent|relevance|score`), `Offset int`.
  - `Query` uses `websearch_to_tsquery('simple', q)`; `relevance` orders by `ts_rank_cd` (offset pagination, max offset 2 000); `score` requires `ProfileID`, joins `research_matches`, orders `(score DESC, job_id DESC)` keyset; `recent` keeps the existing keyset.
  - Workspace mode excludes companies blocked in the workspace and returns `Saved bool` and `Match *domain.Match` per job.
- `func (r *JobRepository) Facets(ctx context.Context, f JobFilter) (Facets, error)`; `type Facets struct { Posted map[string]int /*24h,7d,30d*/; Arrangement, Seniority, Country map[string]int; Technologies, Companies []FacetCount }`; `type FacetCount struct { Value, Label string; Count int }` — each dimension counted with every filter except its own; technologies and companies top 20.
- HTTP: `GET /workspaces/:ws/jobs` (params `q, profile, sort, posted=24h|7d|30d, arrangement, seniority, employmentType, technology (repeat), country (repeat), company (repeat), closed=true, cursor, limit`) and `GET /workspaces/:ws/jobs/facets` (same params minus paging). `/jobs` stays for compatibility. `jobDTO` gains `companyId`, `countryCode`, `closedAt`, `saved`, `match {score, reasons, new}`.
- [ ] **Step 1:** failing tests: `TestSearchRanksTitleAboveDescription`, `TestSearchTurkishCharacters` ("geliştirici" matches), `TestScoreSortKeyset` (pages don't repeat/skip), `TestFacetsExcludeOwnDimension`, `TestBlockedCompaniesHidden`, `TestClosedHiddenByDefault`, handler param validation (`posted=1y` → 422, unknown sort → 422, `sort=score` without profile → 422).
- [ ] **Step 2–4:** FAIL → implement → PASS; `EXPLAIN` the default score query on 100 000 synthetic jobs (one-off, not committed) to confirm index use; commit Milestone 5; push; CI green.

## Milestone 6: UI shell, dashboard, research

### Task 12: Shell, theme, API client types

**Files:**
- Create: `web/src/components/AppShell.tsx` (replaces `Layout.tsx`), `Sidebar.tsx`, `ThemeToggle.tsx`, `web/src/lib/theme.ts`, `web/src/components/__tests__/shell.test.tsx`
- Modify: `web/src/index.css` (`@custom-variant dark (&:where(.dark, .dark *));` + design tokens), `web/src/app/routes.tsx`, `web/src/lib/api.ts` (types: `ResearchProfile`, `ResearchRun`, `StageProgress`, `RunEvent`, `Match`, `Company`, `AdapterReview`, `Facets`, `User.isAdmin`), `messages.ts`, `ui.tsx`, `styles.ts`
- Delete: `web/src/components/Layout.tsx` after routes move.

**Interfaces:**
- `type Theme = "light" | "dark" | "system"`; `function useTheme(): { theme: Theme; resolved: "light" | "dark"; setTheme(t: Theme): void }` — stored in `localStorage["rolescout.theme"]` (try/catch), applies `dark` class on `<html>`, listens to `prefers-color-scheme`.
- Routes: `/` → `DashboardPage`, `/research`, `/research/profiles/:profileId`, `/research/runs/:runId`, `/jobs` (browser), `/saved`, `/applications` (`?view=board|list`), `/applications/:applicationId`, `/companies`, `/settings` (admin only, else NotFound). `/jobs/:jobId` redirects to `/jobs?job=:jobId`.
- Sidebar: Dashboard, Research, Jobs, Saved, Applications, Companies, Settings (admin) with lucide icons; collapses to a top bar + menu button under `md`.
- [ ] **Step 1:** failing tests: nav shows Settings only for admins; theme toggle cycles and sets `html.dark`; mobile menu opens/closes with keyboard; every new message key exists in both locales (extend the existing i18n parity test).
- [ ] **Step 2–4:** FAIL → implement → PASS (`npm test`, `npm run lint`, `npm run build`).

### Task 13: Dashboard and research screens

**Files:**
- Create: `web/src/features/research/api.ts`, `ResearchPage.tsx`, `ProfileForm.tsx`, `RunPage.tsx`, `StageCard.tsx`, `RunLog.tsx`, `__tests__/research.test.tsx`; `web/src/features/dashboard/DashboardPage.tsx`, `__tests__/dashboard.test.tsx`
- Modify: `messages.ts`, `routes.tsx`

**Interfaces:**
- Hooks: `useProfiles()`, `useProfile(id)`, `useSaveProfile()`, `useDeleteProfile()`, `useStartRun(profileId)`, `useRuns(profileId?)`, `useRun(runId)` (`refetchInterval: 2000` while `queued|running`, else false), `useRunEvents(runId, level?)` (accumulates by `after` cursor every 2s while running), `useCancelRun(runId)`.
- RunPage layout (mockup A): left column five `StageCard`s (icon by state: check / spinner / clock / skip / x; progress bar `Done/Total`; counts; `Detail` as chips, e.g. "Greenhouse 640"); right column `RunLog` (monospace, newest at bottom, auto-scroll unless the user scrolled up, "errors only" toggle); header with profile name, state, elapsed time, Cancel button; finished runs show the summary and a "See matches" link → `/jobs?profile=<id>&sort=score`.
- ResearchPage: profile cards with last run state and "Run now"; run history table per profile; ProfileForm (React Hook Form + Zod) with chip inputs for keyword lists, country multi-select (ISO list from `labels.ts`), remote preference radio, seniority checkboxes.
- Dashboard: last run card (state, finished time, new matches, "Run now"), "New today" top 5 matches, application counts by status, upcoming events (existing data).
- [ ] **Step 1:** failing tests: RunPage renders stage states from a mocked run and appends events across two polls; cancel calls the endpoint; polling stops when `succeeded`; ProfileForm validation messages (empty name, > 30 keywords); Dashboard shows "no research yet" empty state with Run button.
- [ ] **Step 2–4:** FAIL → implement → PASS; commit Milestone 6; push; CI green.

## Milestone 7: Job browser, kanban, companies, settings

### Task 14: Three-column job browser

**Files:**
- Create: `web/src/features/jobs/JobsPage.tsx` (replaces `JobSearchPage.tsx`), `FacetPanel.tsx`, `JobList.tsx`, `JobDetailPane.tsx`, `MatchReasons.tsx`, `useJobKeyboard.ts`, `searchParams.ts`; update `__tests__/jobs.test.tsx`
- Modify: `features/jobs/api.ts`, `JobCard.tsx` (compact list item), `messages.ts`
- Delete: `JobSearchPage.tsx`, `JobDetailPage.tsx` (detail lives in the pane; mobile shows the pane full-screen with a back button)

**Interfaces:**
- `function readJobSearch(params: URLSearchParams): JobSearch` / `function writeJobSearch(s: JobSearch): URLSearchParams` in `searchParams.ts`; `JobSearch = { q, profile, sort, posted, arrangement, seniority, technologies[], countries[], companies[], job }`; defaults: first profile and `sort=score` when a profile exists, else `recent`.
- `useWorkspaceJobs(search)` (infinite query on `/workspaces/:ws/jobs`), `useFacets(search)`.
- Keyboard (ignored while typing in inputs): `j`/`k` next/previous (selects and scrolls into view), `s` save toggle, `o` open career page in new tab, `/` focus search.
- Detail pane: title, company, location + country, arrangement, posted, technologies chips, `MatchReasons` (translated reason codes, e.g. `tech` → "5/6 teknolojin eşleşiyor"), Save, "I applied" (creates application then links to it), "Career page ↗" (`rel="noopener noreferrer"`), description text.
- [ ] **Step 1:** failing tests: URL round-trip of `JobSearch`; facet click updates URL and refetches; selecting a job shows the pane; `j`/`k`/`s` behaviour; `/jobs/abc` redirects to `/jobs?job=abc`; closed job shows "no longer listed" badge.
- [ ] **Step 2–4:** FAIL → implement → PASS.

### Task 15: Applications kanban, companies, settings

**Files:**
- Create: `web/src/features/applications/ApplicationsBoard.tsx`, `__tests__/board.test.tsx`; `web/src/features/companies/api.ts`, `CompaniesPage.tsx`, `__tests__/companies.test.tsx`; `web/src/features/settings/SettingsPage.tsx`, `__tests__/settings.test.tsx`
- Modify: `ApplicationsPage.tsx` (list/board toggle via `?view=`), `messages.ts`

**Interfaces:**
- Board: one column per `applicationStatuses`; native HTML5 drag and drop; drop calls existing `PATCH /applications/:id {status}`; a column only accepts drops allowed by `statusTransitions` (others show not-allowed styling); each card has a "Move to…" menu as the keyboard/touch alternative; server 409 shows a toast and reverts.
- Companies: table (name, ATS, status badge, open jobs, last checked, error) with search, ATS and status filters, cursor "load more"; "Add company" dialog (career page URL) → shows `pending`; row action Block/Unblock for this workspace; admins also Disable/Enable globally.
- Settings (admin): adapter list with review state, note, enable/disable (note required to enable, Workday shows a warning that it is not a documented public API); crawl settings read-only table.
- [ ] **Step 1:** failing tests: drag/drop and menu move call PATCH with the right status; disallowed column rejects; companies add validates URL and shows pending row; block toggles; settings enable without note shows validation; non-admin cannot reach `/settings`.
- [ ] **Step 2–4:** FAIL → implement → PASS; commit Milestone 7; push; CI green.

## Milestone 8: Live run, docs, release prep

### Task 16: End-to-end, docs, roadmap

**Files:**
- Modify: `scripts/smoke.sh` (create profile, start run, poll until finished or 120s with the fake-free local stack → only asserts the run reaches `running` and stages progress; full completion not required in CI), `.github/workflows/ci.yml` only if smoke needs env, `README.md` (what RoleScout does now, screenshots not required), `docs/installation.md` (first research run), `docs/operations.md` (crawl load, tiers, retention, cancel, admin adapters), `docs/security.md` (outbound crawling, robots, admin role, review date), `docs/sources/README.md`, `CHANGELOG.md` `[Unreleased]`, `docs/PHASES.md` (Phase 9 status IN_PROGRESS → DONE for this scope with exit evidence)
- [ ] **Step 1:** `make up`, register, run the default profile against the real internet; record in `docs/operations.md`: companies discovered, detected per ATS, jobs collected, matches, duration, failures by reason. Fix what the live run reveals (each fix with a test).
- [ ] **Step 2:** check the UI in the browser at desktop and 375 px, light and dark.
- [ ] **Step 3:** `make lint test`, `npm run build`, `scripts/smoke.sh`, `govulncheck`, `npm audit` all clean.
- [ ] **Step 4:** commit Milestone 8 "Document career page research and finish Phase 9 scope"; push; CI green.

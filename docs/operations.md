# Operations

## Upgrades
1. Read [CHANGELOG.md](../CHANGELOG.md) for the versions you are skipping.
2. Back up the database (see below).
3. Fetch and start the new version:
   ```bash
   git fetch --tags
   git checkout v0.2.0            # the version to install
   ROLESCOUT_VERSION=v0.2.0 docker compose up -d --build --wait
   ```
4. Check `docker compose logs api` for the version line and run
   `scripts/smoke.sh` with your account.

### Migrations
- The API applies pending migrations from `api/migrations` at startup and
  records them in `rolescout_schema_migrations`. A PostgreSQL advisory lock makes
  concurrent API instances apply them one at a time.
- Migrations are forward-only and additive where possible. There are no down
  migrations: to roll back, restore the backup taken before the upgrade and
  check out the previous version.
- If a migration fails, the API exits and logs the error; the database is left
  at the last successful migration because each migration runs in its own
  transaction. Fix the cause, then start the API again.

## Backup and restore
```bash
scripts/backup.sh                     # writes backups/rolescout-<UTC time>.dump
scripts/restore.sh backups/<file>.dump --yes
```
- Backups use `pg_dump` in the custom format (compressed) and are readable
  only by the current user. Copy them off the host; a backup on the same disk
  does not survive disk loss.
- Restore stops the API, worker and web server, replaces the database
  contents in a single transaction and starts the services again. Without
  `--yes` it refuses to run.
- Restore into the same or a newer RoleScout version; the API applies any
  newer migrations on startup.
- Schedule backups with cron, for example daily at 03:00:
  `0 3 * * * cd /opt/rolescout && scripts/backup.sh >/dev/null`, and delete
  old files with your retention policy.
- CI verifies backup and restore on every change by deleting data and
  restoring it.

## Career page research
- **What runs when.** Each research profile runs every `schedule_hours`
  (12 by default) and on **Run now**. A new profile runs at the worker's next
  tick. Only one run executes the shared stages (discovery, detection,
  collection) at a time; others wait and then reuse the fresh results.
- **Discovery** runs at most every `DISCOVERY_INTERVAL` (7 days). It reads
  the seed list, Hacker News "Who is hiring?" threads (12 months on the first
  run, then new ones and the current month) and public company lists.
- **Collection** syncs company sources that are due. Sources refresh every
  12 hours when the company has a matching job, every 3 days when it has
  other open jobs, and weekly when it has none. Failing companies back off
  from 1 hour up to 7 days; jobs of a source that keeps failing for 7 days
  are closed.
- **Load.** Requests identify RoleScout, follow robots.txt (including
  `Crawl-delay`; Lever asks for one second), and are limited per host
  (`CRAWL_HOST_INTERVAL`) and per job board API (`CRAWL_VENDOR_CONCURRENCY`).
  Lower `CRAWL_CONCURRENCY` on a small machine or connection.
- **Cancel** a run on its page; it stops at the next progress report. A run
  left behind by a stopped worker is marked failed after 15 minutes.
- **Retention.** Closed jobs nobody saved or applied to are deleted after
  `CLOSED_JOB_RETENTION` (90 days); run logs after 30 days.
- **Companies.** `docker compose run --rm admin companies list -status unreachable`
  shows companies that fail and why; disable one for everyone under
  **Companies** (administrator) or with the API.
- **Seeds.** Before a release, check the seed list:
  `docker compose run --rm admin seeds check` (entries without a supported
  board or open jobs are reported as `DEAD`).

LIVE_RUN_PLACEHOLDER

## Monitoring
- **Health**: `/healthz` (process up) and `/readyz` (database reachable) on the
  API port; the web container has its own `/healthz`. Compose health checks
  use them, and `docker compose ps` shows the result.
- **Logs**: the API and worker write JSON lines to stdout
  (`docker compose logs -f api worker`). API request lines contain
  `request_id`, `method`, `route`, `status` and `duration_ms`; responses carry
  the same ID in `X-Request-ID`. Unexpected errors are logged with the request
  ID and returned to clients only as `internal`.
- **Metrics**: `/metrics` on the API port, in Prometheus text format:
  `rolescout_http_requests_total`, `rolescout_http_request_duration_seconds`,
  `rolescout_http_requests_in_flight`, `rolescout_db_connections` and
  `rolescout_db_acquire_wait_seconds_total`. It is not exposed through the web
  server.
- **Ingestion**: `docker compose run --rm admin sources list` shows each
  source's last run; `admin sources runs -key KEY` shows recent runs with
  counts and errors. Worker log lines `source sync failed` include the error.

## Troubleshooting
| Symptom | Check |
| --- | --- |
| `docker compose up` says `Set POSTGRES_PASSWORD` | `.env` is missing or has no password. |
| API keeps restarting | `docker compose logs api`; usually a database URL or migration error. |
| Sign-in works but the session is lost | `COOKIE_SECURE=true` while serving plain HTTP. |
| `429 rate_limited` for everyone | `TRUSTED_PROXIES` does not include your reverse proxy, so all clients share its IP. |
| A source never syncs | It must be reviewed and enabled; see `admin sources list`. |

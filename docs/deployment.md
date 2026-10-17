# Production Deployment

RoleScout is designed for a single host running Docker Compose. This guide
puts it on the internet behind HTTPS.

## Recommended setup
```
internet ──HTTPS──> Caddy (80/443) ──> web: nginx (static app, /api proxy) ──> api ──> postgres
                                                                    worker ──────────────┘
```
`deploy/compose.https.yaml` adds Caddy, which obtains and renews TLS
certificates automatically, and sets `COOKIE_SECURE=true`.

1. Point a DNS record (for example `jobs.example.com`) at the server and allow
   inbound TCP 80 and 443 (and UDP 443 for HTTP/3).
2. Install as in [installation](installation.md), then add to `.env`:
   ```
   ROLESCOUT_DOMAIN=jobs.example.com
   ```
3. Start with the override:
   ```bash
   docker compose -f compose.yaml -f deploy/compose.https.yaml up -d --build --wait
   ```
   Use the same `-f` flags for every later `docker compose` command, or set
   `COMPOSE_FILE=compose.yaml:deploy/compose.https.yaml` in `.env`.
4. Open `https://jobs.example.com`, create the first account and keep
   `ALLOW_REGISTRATION=false` unless others should sign up.

### Using another reverse proxy
Any proxy works if it:
- terminates HTTPS and forwards to the web container (port 8080 in the
  container, `WEB_PORT` on the host),
- sets `X-Forwarded-For`; nginx trusts it from private networks and passes the
  client address to the API, which rate limits by it. If your proxy reaches
  RoleScout from a public address, extend `set_real_ip_from` in
  `web/nginx.conf`.
- and you set `COOKIE_SECURE=true`.

## Checklist
- [ ] `POSTGRES_PASSWORD` is long and random; `.env` is readable only by the
      operator (`chmod 600 .env`).
- [ ] Served over HTTPS with `COOKIE_SECURE=true`.
- [ ] `ALLOW_REGISTRATION=false` after the intended accounts exist.
- [ ] Only ports 80 and 443 are reachable from outside; PostgreSQL, the API
      and `/metrics` stay on 127.0.0.1 or a private network.
- [ ] Daily backups copied off the host, and a restore tested
      ([operations](operations.md#backup-and-restore)).
- [ ] Health checks and logs are monitored; optionally scrape `/metrics`.
- [ ] Every enabled job source has a recorded review
      ([sources](sources/README.md)).
- [ ] Host and Docker updates are applied regularly; rebuild images with
      `docker compose build --pull` to pick up base image fixes.

## Scaling notes
- One API instance handles typical personal and team use. Running several
  needs a load balancer, and the sign-in rate limits then apply per instance.
- Run one worker. Several workers are safe (a source cannot be synced twice at
  once) but do not speed up a single source.
- Size PostgreSQL memory and disk for your listing volume; job search uses
  indexes on recency and technologies.

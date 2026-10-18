# Installation

This guide installs RoleScout on one machine with Docker Compose. For a
public server, continue with [deployment](deployment.md) afterwards.

## Requirements
- Docker Engine 24 or newer with the Compose plugin (`docker compose version`)
- 2 GB of RAM and a few GB of disk for images and data
- Git

## Install
```bash
git clone https://github.com/0furkancolak/rolescout.git
cd rolescout
cp .env.example .env
```

Edit `.env` and set `POSTGRES_PASSWORD` to a long random value, for example
the output of `openssl rand -hex 24`. Update the password in `DATABASE_URL`
too if you plan to run the API outside Docker. Then start everything:

```bash
docker compose up -d --build --wait
```

`--wait` returns once PostgreSQL, the API and the web server report healthy.
The API applies database migrations on startup.

| Service | Address |
| --- | --- |
| Web app | http://127.0.0.1:4173 |
| API liveness / readiness | http://127.0.0.1:8080/healthz, /readyz |
| API metrics (Prometheus) | http://127.0.0.1:8080/metrics |

All ports are bound to 127.0.0.1. To reach RoleScout from other machines, put
it behind HTTPS as described in [deployment](deployment.md).

## First steps
1. Open the web app and create your account. The first account can always
   register; later sign-ups need `ALLOW_REGISTRATION=true` in `.env` followed
   by `docker compose up -d`.
2. Your workspace starts with a "Full Stack Developer" research profile. The
   worker starts its first research within a minute: it discovers companies,
   identifies their career pages, collects their jobs and scores them. Follow
   it under **Research**; the first run reads about a year of Hacker News
   "Who is hiring?" threads and takes a while (see
   [operations](operations.md#career-page-research)). Change the profile or
   add others under **Research**, add companies by career page URL under
   **Companies**.
3. The first account is the administrator. **Settings** shows which job
   board systems are enabled; Workday stays off until you review it (see
   [sources](sources/README.md)).
4. Check the installation end to end:
   ```bash
   scripts/smoke.sh
   ```
   Without `SMOKE_EMAIL` and `SMOKE_PASSWORD` it registers a test account,
   which only works while registration is open. To use your own account:
   `SMOKE_EMAIL=you@example.com SMOKE_PASSWORD=... scripts/smoke.sh`. It
   creates a workspace named "Smoke test" with a research profile, starts a
   run and cancels it again.

## Optional: local AI
To let a small model on your own computer read job descriptions, follow
[local AI](local-ai.md). It is off by default and the installation works the
same without it.

## Stopping and data
- `docker compose stop` stops the services; `docker compose up -d --wait`
  starts them again. Data lives in the `rolescout_postgres_data` volume and
  survives restarts and rebuilds.
- `docker compose down` removes containers but keeps the volume.
  `docker compose down -v` **deletes all data**.

Next: [operations](operations.md) for upgrades, backups and monitoring.

#!/bin/sh
# End-to-end smoke test against a running installation, through the web proxy.
#
# Usage: scripts/smoke.sh [base-url]        (default http://127.0.0.1:4173)
#
# It signs in with SMOKE_EMAIL and SMOKE_PASSWORD when both are set. Otherwise
# it registers a new account, which only works while registration is open
# (always for the first account). It creates a workspace named "Smoke test".
set -eu

base="${1:-http://127.0.0.1:4173}"
jar="$(mktemp)"
body="$(mktemp)"
trap 'rm -f "$jar" "$body"' EXIT
failures=0

# check NAME EXPECTED_STATUS METHOD PATH [JSON]
check() {
  name="$1" want="$2" method="$3" path="$4" data="${5:-}"
  if [ -n "$data" ]; then
    got=$(curl -sS -o "$body" -w '%{http_code}' -b "$jar" -c "$jar" -X "$method" \
      -H 'Content-Type: application/json' --data "$data" "$base$path")
  else
    got=$(curl -sS -o "$body" -w '%{http_code}' -b "$jar" -c "$jar" -X "$method" "$base$path")
  fi
  if [ "$got" = "$want" ]; then
    echo "ok   $name"
  else
    echo "FAIL $name: $method $path returned $got, want $want"
    head -c 300 "$body"; echo
    failures=$((failures + 1))
  fi
}

# field NAME: prints a string field from the last JSON response.
field() {
  sed -n "s/.*\"$1\":\"\([^\"]*\)\".*/\1/p" "$body" | head -n 1
}

check "web proxy health" 200 GET /healthz
check "web app shell" 200 GET /jobs
grep -q '<div id="root">' "$body" || { echo "FAIL web app shell has no root element"; failures=$((failures + 1)); }
check "api requires a session" 401 GET /api/v1/auth/me

if [ -n "${SMOKE_EMAIL:-}" ] && [ -n "${SMOKE_PASSWORD:-}" ]; then
  check "sign in" 200 POST /api/v1/auth/login "{\"email\":\"$SMOKE_EMAIL\",\"password\":\"$SMOKE_PASSWORD\"}"
else
  email="smoke-$(date +%s)-$$@example.com"
  password="smoke-$(od -An -N12 -tx1 /dev/urandom | tr -d ' \n')"
  check "register" 201 POST /api/v1/auth/register "{\"email\":\"$email\",\"password\":\"$password\",\"display_name\":\"Smoke Test\",\"locale\":\"en\"}"
fi

check "current user" 200 GET /api/v1/auth/me
check "list workspaces" 200 GET /api/v1/workspaces
check "create workspace" 201 POST /api/v1/workspaces '{"name":"Smoke test"}'
ws=$(field id)
check "search jobs" 200 GET '/api/v1/jobs?limit=5'
job=$(field id)
check "list applications" 200 GET "/api/v1/workspaces/$ws/applications"
check "create tag" 201 POST "/api/v1/workspaces/$ws/tags" '{"name":"smoke"}'
if [ -n "$job" ]; then
  check "save job" 204 PUT "/api/v1/workspaces/$ws/saved-jobs/$job"
  check "track application" 201 POST "/api/v1/workspaces/$ws/applications" "{\"job_id\":\"$job\"}"
  app=$(field id)
  check "record attempt" 201 POST "/api/v1/workspaces/$ws/applications/$app/attempts" "{\"applied_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"channel\":\"smoke\"}"
  check "application history" 200 GET "/api/v1/workspaces/$ws/applications/$app/events"
else
  echo "skip job and application checks: no jobs have been imported"
fi
check "research profiles" 200 GET "/api/v1/workspaces/$ws/research-profiles"
check "create research profile" 201 POST "/api/v1/workspaces/$ws/research-profiles" \
  '{"name":"Smoke profile","title_keywords":["full stack"],"technologies":["react"],"schedule_hours":336}'
profile=$(field id)
check "start research run" 202 POST "/api/v1/workspaces/$ws/research-profiles/$profile/runs"
run=$(field id)
check "research run progress" 200 GET "/api/v1/workspaces/$ws/research-runs/$run"
check "research run log" 200 GET "/api/v1/workspaces/$ws/research-runs/$run/events?after=0"
# Stop the run again so a smoke test never leaves a crawl behind.
check "cancel research run" 202 POST "/api/v1/workspaces/$ws/research-runs/$run/cancel"
check "browse matches" 200 GET "/api/v1/workspaces/$ws/jobs?profile=$profile&sort=score&limit=5"
check "job facets" 200 GET "/api/v1/workspaces/$ws/jobs/facets"
check "company catalog" 200 GET "/api/v1/workspaces/$ws/companies?limit=5"
check "other workspaces are hidden" 404 GET /api/v1/workspaces/00000000-0000-4000-8000-000000000000/applications
check "sign out" 204 POST /api/v1/auth/logout
check "session revoked" 401 GET /api/v1/auth/me

if [ "$failures" -gt 0 ]; then
  echo "$failures check(s) failed"
  exit 1
fi
echo "all checks passed"

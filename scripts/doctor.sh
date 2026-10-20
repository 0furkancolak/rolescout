#!/bin/sh
# Checks that this computer can run the development environment and says what
# to fix. Exit status 1 when something required is missing.
#
# Usage: scripts/doctor.sh           (or: make doctor)
#        scripts/doctor.sh --check   print only the problems (used by make dev)
set -u

quiet=0
[ "${1:-}" = "--check" ] && quiet=1
fail=0
say()  { [ "$quiet" -eq 1 ] || printf '%s\n' "$1"; }
ok()   { [ "$quiet" -eq 1 ] || printf '  ok    %s\n' "$1"; }
warn() { [ "$quiet" -eq 1 ] || printf '  warn  %s\n' "$1"; }
bad()  { printf '  FAIL  %s\n' "$1"; fail=1; }

say "Tools"
if command -v docker >/dev/null 2>&1; then
  if docker info >/dev/null 2>&1; then ok "docker is running"; else bad "docker is installed but not running: start Docker Desktop or OrbStack"; fi
  if docker compose version >/dev/null 2>&1; then ok "docker compose available"; else bad "docker compose plugin missing"; fi
else
  bad "docker not found: install Docker Desktop or OrbStack"
fi
if command -v node >/dev/null 2>&1; then
  major=$(node -p 'process.versions.node.split(".")[0]')
  if [ "$major" -ge 22 ]; then ok "node $(node -v)"; else bad "node $(node -v) is too old: use 22 or newer"; fi
else
  bad "node not found: install Node.js 22 or newer"
fi
if command -v go >/dev/null 2>&1; then ok "$(go version | cut -d' ' -f3) (only needed to run Go tests outside Docker)"; else warn "go not found: Go tests cannot run on this computer (the app still runs in Docker)"; fi

say "Configuration"
if [ -f .env ]; then
  ok ".env exists"
  if grep -q 'replace-with-a-local-password' .env; then bad ".env still has the placeholder password: delete .env and run make env"; fi
else
  warn ".env is missing: make dev creates it"
fi

say "Ports"
port_of() { (. ./.env 2>/dev/null; eval "echo \${$1:-$2}") 2>/dev/null || echo "$2"; }
for spec in "WEB_DEV_PORT:5173:web app (make dev)" "API_HOST_PORT:8080:API" "POSTGRES_PORT:5432:PostgreSQL"; do
  var=${spec%%:*}; rest=${spec#*:}; def=${rest%%:*}; label=${rest#*:}
  p=$(port_of "$var" "$def")
  holder=$(lsof -nP -iTCP:"$p" -sTCP:LISTEN 2>/dev/null | awk 'NR==2 {print $1}')
  if [ -z "$holder" ]; then
    ok "port $p ($label) is free"
  elif command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}} {{.Ports}}' 2>/dev/null | grep -E "rolescout.*:$p->" >/dev/null; then
    ok "port $p ($label) is used by this project's containers"
  else
    free=$p; while lsof -nP -iTCP:"$free" -sTCP:LISTEN >/dev/null 2>&1; do free=$((free + 1)); done
    case $var in
      WEB_DEV_PORT) warn "port $p ($label) is taken by $holder: run make dev WEB_DEV_PORT=$free" ;;
      *) bad "port $p ($label) is taken by $holder: add $var=$free to .env" ;;
    esac
  fi
done

say "AI (optional)"
if [ -f .env ] && grep -q '^LLM_API_KEY=.\+' .env; then
  ok "LLM_API_KEY is set in .env (the web app's Settings → AI overrides it)"
else
  ok "no API key in .env: set one in the web app under Settings → AI, or run Ollama"
fi
if curl -fsS -m 2 http://127.0.0.1:11434/api/tags >/dev/null 2>&1; then ok "Ollama answers on :11434"; else warn "Ollama is not running (fine when you use a hosted service)"; fi

if [ "$fail" -eq 0 ]; then say "All required checks passed."; else echo "Fix the FAIL lines above, then run again."; fi
exit "$fail"

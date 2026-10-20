#!/usr/bin/env bash
# Deploys the committed HEAD to a Docker host over SSH, private by default.
#
#   scripts/deploy-server.sh nafru-lan
#   ROLESCOUT_SERVICES="api web" scripts/deploy-server.sh nafru-lan   # leave the worker (and its research run) alone
#
# No manual .env: on first run the server generates its own secrets. The web
# container listens on 127.0.0.1 of the server only; reach it over Tailscale
# (`tailscale serve`) or an SSH tunnel. Nothing is added to a public proxy.
set -euo pipefail

HOST="${1:?usage: deploy-server.sh <ssh-host>}"
DIR="${ROLESCOUT_REMOTE_DIR:-rolescout}"
PORT="${ROLESCOUT_WEB_PORT:-4173}"
SERVICES="${ROLESCOUT_SERVICES:-}"

cd "$(git rev-parse --show-toplevel)"
# Extract into a fresh directory so files deleted in the repository do not
# linger on the server, then keep the server's own .env.
git archive --format=tar HEAD | ssh "$HOST" "rm -rf '$DIR.new' && mkdir -p '$DIR.new' && tar -x -C '$DIR.new' \
  && { [ ! -f '$DIR/.env' ] || cp -p '$DIR/.env' '$DIR.new/.env'; } \
  && mkdir -p '$DIR' && rm -rf '$DIR.old' && mv '$DIR' '$DIR.old' && mv '$DIR.new' '$DIR'"

ssh "$HOST" bash -s -- "$DIR" "$PORT" $SERVICES <<'REMOTE'
set -euo pipefail
cd "$1"
if [ ! -f .env ]; then
  umask 077
  {
    echo "POSTGRES_PASSWORD=$(head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 32)"
    echo "WEB_PORT=$2"
    echo "API_HOST_PORT=18080"
    echo "POSTGRES_PORT=15432"
    echo "COOKIE_SECURE=true"
    echo "ALLOW_REGISTRATION=false"
    echo "COMPOSE_PROJECT_NAME=rolescout"
  } > .env
fi
docker compose up -d --build --wait "${@:3}"
docker compose ps --format 'table {{.Service}}\t{{.Status}}'
REMOTE

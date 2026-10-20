#!/usr/bin/env bash
# Deploys the committed HEAD to a Docker host over SSH, private by default.
#
#   scripts/deploy-server.sh nafru-lan
#
# No manual .env: on first run the server generates its own secrets. The web
# container listens on 127.0.0.1 of the server only; reach it over Tailscale
# (`tailscale serve`) or an SSH tunnel. Nothing is added to a public proxy.
set -euo pipefail

HOST="${1:?usage: deploy-server.sh <ssh-host>}"
DIR="${ROLESCOUT_REMOTE_DIR:-rolescout}"
PORT="${ROLESCOUT_WEB_PORT:-4173}"

cd "$(git rev-parse --show-toplevel)"
git archive --format=tar HEAD | ssh "$HOST" "mkdir -p '$DIR' && tar -x -C '$DIR'"

ssh "$HOST" bash -s -- "$DIR" "$PORT" <<'REMOTE'
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
docker compose up -d --build --wait
docker compose ps --format 'table {{.Service}}\t{{.Status}}'
REMOTE

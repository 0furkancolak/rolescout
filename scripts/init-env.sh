#!/bin/sh
# Creates .env from .env.example with a random database password.
# Does nothing when .env already exists.
#
# Usage: scripts/init-env.sh [path]        (default .env)
set -eu

target="${1:-.env}"
if [ -e "$target" ]; then
  exit 0
fi

if command -v openssl >/dev/null 2>&1; then
  password="$(openssl rand -hex 24)"
else
  password="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
fi

umask 077
sed "s/replace-with-a-local-password/$password/g" .env.example > "$target"
echo "Created $target with a random POSTGRES_PASSWORD."

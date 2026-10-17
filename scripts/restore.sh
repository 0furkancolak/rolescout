#!/bin/sh
# Restores a backup made by scripts/backup.sh into the Compose database,
# replacing its current contents.
# Usage: scripts/restore.sh <backup-file> --yes
set -eu

cd "$(dirname "$0")/.."
file="${1:-}"
if [ -z "$file" ] || [ ! -f "$file" ]; then
  echo "usage: scripts/restore.sh <backup-file> --yes" >&2
  exit 2
fi
if [ "${2:-}" != "--yes" ]; then
  echo "This replaces all data in the database with $file." >&2
  echo "Run again with --yes to continue." >&2
  exit 2
fi

# Stop writers so the restore sees a quiet database. They are started again
# below, also when the restore fails.
docker compose stop api worker web >/dev/null
trap 'docker compose up -d api worker web >/dev/null' EXIT

docker compose exec -T postgres sh -c \
  'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --single-transaction --exit-on-error' < "$file"
echo "Restored $file"

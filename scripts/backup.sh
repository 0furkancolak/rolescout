#!/bin/sh
# Writes a compressed PostgreSQL backup of the Compose database.
# Usage: scripts/backup.sh [output-file]
set -eu

cd "$(dirname "$0")/.."
out="${1:-backups/rolescout-$(date -u +%Y%m%dT%H%M%SZ).dump}"
mkdir -p "$(dirname "$out")"

# pg_dump runs inside the postgres container, so no client tools or
# credentials are needed on the host. The custom format is compressed and
# restorable with pg_restore.
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner' > "$out.partial"
mv "$out.partial" "$out"
chmod 600 "$out"
echo "Backup written to $out ($(wc -c < "$out" | tr -d ' ') bytes)"

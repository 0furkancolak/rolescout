package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var migrationName = regexp.MustCompile(`^([0-9]{14})_[a-z0-9_]+\.sql$`)

// migrationLockID is an arbitrary constant shared by every RoleScout process so
// concurrent API replicas apply migrations one at a time.
const migrationLockID int64 = 7_202_610_171_200

// RunMigrations applies unapplied timestamped SQL migrations in order while
// holding a session-level advisory lock.
// Files must use YYYYMMDDHHMMSS_description.sql naming.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		// Use a fresh context so the lock is released even if ctx was cancelled.
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockID)
	}()

	return applyMigrations(ctx, conn.Conn(), dir)
}

func applyMigrations(ctx context.Context, conn *pgx.Conn, dir string) error {
	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS rolescout_schema_migrations (
			version BIGINT PRIMARY KEY,
			filename TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create migration tracking table: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations directory %q: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		if !migrationName.MatchString(entry.Name()) {
			return fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		match := migrationName.FindStringSubmatch(name)
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return fmt.Errorf("parse migration version %q: %w", name, err)
		}

		var applied bool
		err = conn.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM rolescout_schema_migrations WHERE version = $1)`,
			version,
		).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}

		if _, err = tx.Exec(ctx, string(sqlBytes)); err == nil {
			_, err = tx.Exec(ctx,
				`INSERT INTO rolescout_schema_migrations (version, filename) VALUES ($1, $2)`,
				version, name,
			)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

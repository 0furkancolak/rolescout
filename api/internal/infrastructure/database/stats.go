package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/rolescout/api/internal/sysstat"
)

// StatsRepository stores service memory samples and reads Postgres's own
// memory settings.
type StatsRepository struct{ pool *pgxpool.Pool }

func NewStatsRepository(pool *pgxpool.Pool) *StatsRepository { return &StatsRepository{pool: pool} }

func (r *StatsRepository) Upsert(ctx context.Context, service string, s sysstat.Sample) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO service_stats (service, used_bytes, limit_bytes, heap_bytes, goroutines)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (service) DO UPDATE SET used_bytes = EXCLUDED.used_bytes, limit_bytes = EXCLUDED.limit_bytes,
			heap_bytes = EXCLUDED.heap_bytes, goroutines = EXCLUDED.goroutines, updated_at = NOW()`,
		service, int64(s.UsedBytes), int64(s.LimitBytes), int64(s.HeapBytes), s.Goroutines)
	return translate(err, "store service stats")
}

func (r *StatsRepository) Samples(ctx context.Context) (map[string]sysstat.Row, error) {
	rows, err := r.pool.Query(ctx, `SELECT service, used_bytes, limit_bytes, heap_bytes, goroutines, updated_at FROM service_stats`)
	if err != nil {
		return nil, translate(err, "list service stats")
	}
	defer rows.Close()
	out := map[string]sysstat.Row{}
	for rows.Next() {
		var name string
		var used, limit, heap int64
		var row sysstat.Row
		if err := rows.Scan(&name, &used, &limit, &heap, &row.Goroutines, &row.UpdatedAt); err != nil {
			return nil, translate(err, "scan service stats")
		}
		row.UsedBytes, row.LimitBytes, row.HeapBytes = uint64(used), uint64(limit), uint64(heap)
		out[name] = row
	}
	return out, translate(rows.Err(), "list service stats")
}

// Postgres reads the memory-related settings (in bytes) and the database size.
func (r *StatsRepository) Postgres(ctx context.Context) (sysstat.PG, error) {
	var p sysstat.PG
	var shared, work, maint, conns, av int64
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT setting::bigint * 8192 FROM pg_settings WHERE name = 'shared_buffers'),
			(SELECT setting::bigint * 1024 FROM pg_settings WHERE name = 'work_mem'),
			(SELECT setting::bigint * 1024 FROM pg_settings WHERE name = 'maintenance_work_mem'),
			(SELECT setting::bigint FROM pg_settings WHERE name = 'max_connections'),
			(SELECT setting::bigint FROM pg_settings WHERE name = 'autovacuum_max_workers'),
			(SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()),
			pg_database_size(current_database())`).
		Scan(&shared, &work, &maint, &conns, &av, &p.Connections, &p.DatabaseBytes)
	if err != nil {
		return p, translate(err, "read postgres memory settings")
	}
	p.SharedBuffers, p.WorkMem, p.MaintenanceWorkMem = uint64(shared), uint64(work), uint64(maint)
	p.MaxConnections, p.AutovacuumWorkers = int(conns), int(av)
	return p, nil
}

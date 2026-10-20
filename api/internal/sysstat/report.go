package sysstat

import (
	"context"
	"os"
	"strconv"
	"time"
)

// Row is a stored sample with its age.
type Row struct {
	Sample
	UpdatedAt time.Time
}

// PG is Postgres's memory configuration and size.
type PG struct {
	SharedBuffers      uint64
	WorkMem            uint64
	MaintenanceWorkMem uint64
	MaxConnections     int
	AutovacuumWorkers  int
	Connections        int
	DatabaseBytes      uint64
}

// Ceiling estimates the most Postgres is configured to use: the shared cache,
// one work_mem per connection (a query can use several, so this is a floor for
// heavy queries), the autovacuum workers and ~6 MiB of process overhead each.
func (p PG) Ceiling() uint64 {
	return p.SharedBuffers + uint64(p.MaxConnections)*(p.WorkMem+6<<20) + uint64(p.AutovacuumWorkers)*p.MaintenanceWorkMem
}

// Reader is what the report reads.
type Reader interface {
	Samples(ctx context.Context) (map[string]Row, error)
	Postgres(ctx context.Context) (PG, error)
}

// Service is one line of the report. Used is nil when it cannot be measured.
type Service struct {
	Name      string     `json:"name"`
	Used      *uint64    `json:"used_bytes"`
	Limit     *uint64    `json:"limit_bytes"`
	Heap      *uint64    `json:"heap_bytes,omitempty"`
	Estimated bool       `json:"estimated"`
	Stale     bool       `json:"stale"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Note      string     `json:"note,omitempty"`
}

// Report is the whole installation.
type Report struct {
	HostTotal uint64    `json:"host_total_bytes"`
	Services  []Service `json:"services"`
	Postgres  PGInfo    `json:"postgres"`
}

// PGInfo is shown beside the Postgres line.
type PGInfo struct {
	SharedBuffers  uint64 `json:"shared_buffers_bytes"`
	WorkMem        uint64 `json:"work_mem_bytes"`
	MaxConnections int    `json:"max_connections"`
	Connections    int    `json:"connections"`
	DatabaseBytes  uint64 `json:"database_bytes"`
}

// Service builds reports.
type Builder struct {
	Reader Reader
	// Self is the name of the process that calls Report (it is measured live).
	Self string
	// Limits are operator-set caps in bytes by service (postgres, web), shown
	// for services that cannot measure themselves.
	Limits map[string]uint64
	Now    func() time.Time
}

const staleAfter = 90 * time.Second

func ptr[T any](v T) *T { return &v }

func (b *Builder) Report(ctx context.Context) (Report, error) {
	now := time.Now
	if b.Now != nil {
		now = b.Now
	}
	rows, err := b.Reader.Samples(ctx)
	if err != nil {
		return Report{}, err
	}
	pg, err := b.Reader.Postgres(ctx)
	if err != nil {
		return Report{}, err
	}
	live := Take()
	rows[b.Self] = Row{Sample: live, UpdatedAt: now()}

	out := Report{HostTotal: HostTotal(), Postgres: PGInfo{
		SharedBuffers: pg.SharedBuffers, WorkMem: pg.WorkMem, MaxConnections: pg.MaxConnections,
		Connections: pg.Connections, DatabaseBytes: pg.DatabaseBytes,
	}}
	for _, name := range []string{"api", "worker"} {
		s := Service{Name: name}
		if r, ok := rows[name]; ok {
			s.Used = ptr(r.UsedBytes)
			if r.LimitBytes > 0 {
				s.Limit = ptr(r.LimitBytes)
			}
			s.Heap = ptr(r.HeapBytes)
			s.UpdatedAt = ptr(r.UpdatedAt)
			s.Stale = now().Sub(r.UpdatedAt) > staleAfter
		} else {
			s.Stale = true
		}
		out.Services = append(out.Services, s)
	}
	pgSvc := Service{Name: "postgres", Estimated: true, Used: nil}
	ceil := pg.Ceiling()
	pgSvc.Limit = ptr(ceil)
	pgSvc.Note = "ceiling"
	if l, ok := b.Limits["postgres"]; ok && l > 0 {
		pgSvc.Limit = ptr(l)
		pgSvc.Note = "container"
	}
	out.Services = append(out.Services, pgSvc)
	web := Service{Name: "web", Estimated: true}
	if l, ok := b.Limits["web"]; ok && l > 0 {
		web.Limit = ptr(l)
	}
	out.Services = append(out.Services, web)
	return out, nil
}

// LimitsFromEnv reads LIMIT_<SERVICE>_MB variables (Compose sets them from its
// mem_limit values) into bytes.
func LimitsFromEnv() map[string]uint64 {
	out := map[string]uint64{}
	for _, name := range []string{"postgres", "web", "api", "worker"} {
		if v, err := strconv.ParseUint(os.Getenv("LIMIT_"+upper(name)+"_MB"), 10, 64); err == nil && v > 0 {
			out[name] = v << 20
		}
	}
	return out
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

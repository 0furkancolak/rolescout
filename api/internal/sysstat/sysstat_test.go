package sysstat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCgroupV2UsedExcludesReclaimableCache(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "memory.current", "300000000\n")
	write(t, dir, "memory.max", "536870912\n")
	write(t, dir, "memory.stat", "anon 1\ninactive_file 100000000\n")
	used, limit := cgroupMemory(dir)
	if used != 200000000 || limit != 536870912 {
		t.Fatalf("used=%d limit=%d", used, limit)
	}
}

func TestCgroupV2NoLimitReadsAsZero(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "memory.current", "1000\n")
	write(t, dir, "memory.max", "max\n")
	if _, limit := cgroupMemory(dir); limit != 0 {
		t.Fatalf("limit = %d", limit)
	}
}

func TestCgroupV1(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "memory/memory.usage_in_bytes", "900\n")
	write(t, dir, "memory/memory.limit_in_bytes", "9223372036854771712\n")
	write(t, dir, "memory/memory.stat", "total_inactive_file 400\n")
	used, limit := cgroupMemory(dir)
	if used != 500 || limit != 0 {
		t.Fatalf("used=%d limit=%d", used, limit)
	}
}

func TestPostgresCeiling(t *testing.T) {
	p := PG{SharedBuffers: 128 << 20, WorkMem: 4 << 20, MaintenanceWorkMem: 32 << 20, MaxConnections: 40, AutovacuumWorkers: 2}
	want := uint64(128<<20) + 40*uint64(10<<20) + 2*uint64(32<<20)
	if p.Ceiling() != want {
		t.Fatalf("ceiling = %d, want %d", p.Ceiling(), want)
	}
}

type fakeReader struct{ rows map[string]Row }

func (f fakeReader) Samples(context.Context) (map[string]Row, error) { return f.rows, nil }
func (f fakeReader) Postgres(context.Context) (PG, error) {
	return PG{SharedBuffers: 128 << 20, WorkMem: 4 << 20, MaintenanceWorkMem: 32 << 20, MaxConnections: 40, AutovacuumWorkers: 2, Connections: 7}, nil
}

func TestReportMarksAMissingOrOldWorkerStale(t *testing.T) {
	now := time.Now()
	b := &Builder{Reader: fakeReader{rows: map[string]Row{"worker": {Sample: Sample{UsedBytes: 5}, UpdatedAt: now.Add(-5 * time.Minute)}}}, Self: "api",
		Limits: map[string]uint64{"postgres": 1 << 30, "web": 128 << 20}, Now: func() time.Time { return now }}
	r, err := b.Report(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]Service{}
	for _, s := range r.Services {
		by[s.Name] = s
	}
	if by["api"].Used == nil || by["api"].Stale {
		t.Errorf("api = %+v", by["api"])
	}
	if !by["worker"].Stale || by["worker"].Used == nil {
		t.Errorf("worker = %+v", by["worker"])
	}
	if by["postgres"].Used != nil || *by["postgres"].Limit != 1<<30 || by["postgres"].Note != "container" {
		t.Errorf("postgres = %+v", by["postgres"])
	}
	if r.Postgres.Connections != 7 || *by["web"].Limit != 128<<20 {
		t.Errorf("report = %+v", r)
	}
	// Without a container limit, Postgres shows its configured ceiling.
	b.Limits = nil
	r, _ = b.Report(context.Background())
	for _, s := range r.Services {
		if s.Name == "postgres" && (s.Note != "ceiling" || *s.Limit != (PG{SharedBuffers: 128 << 20, WorkMem: 4 << 20, MaintenanceWorkMem: 32 << 20, MaxConnections: 40, AutovacuumWorkers: 2}).Ceiling()) {
			t.Errorf("postgres = %+v", s)
		}
	}
}

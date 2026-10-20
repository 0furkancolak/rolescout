// Package sysstat measures how much memory this installation uses and may use:
// the process's own container (cgroup), the Postgres configuration ceiling and
// the limits the operator set in Compose.
package sysstat

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Sample is one service's memory at a moment.
type Sample struct {
	UsedBytes  uint64 // container memory without reclaimable page cache
	LimitBytes uint64 // 0 when the container has no limit
	HeapBytes  uint64 // Go heap in use
	Goroutines int
}

// Take measures the current process's container.
func Take() Sample {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s := Sample{HeapBytes: ms.HeapAlloc, Goroutines: runtime.NumGoroutine()}
	s.UsedBytes, s.LimitBytes = cgroupMemory("/sys/fs/cgroup")
	if s.UsedBytes == 0 {
		s.UsedBytes = ms.Sys - ms.HeapReleased
	}
	return s
}

// cgroupMemory reads cgroup v2, then v1, under root. Page cache that the
// kernel can drop (inactive file) is not counted, like `docker stats`.
func cgroupMemory(root string) (used, limit uint64) {
	if cur, ok := readUint(root + "/memory.current"); ok {
		used = cur
		if max, ok := readUint(root + "/memory.max"); ok {
			limit = max
		}
		if inactive, ok := statValue(root+"/memory.stat", "inactive_file"); ok && inactive < used {
			used -= inactive
		}
		return used, limit
	}
	if cur, ok := readUint(root + "/memory/memory.usage_in_bytes"); ok {
		used = cur
		if max, ok := readUint(root + "/memory/memory.limit_in_bytes"); ok && max < 1<<60 {
			limit = max
		}
		if inactive, ok := statValue(root+"/memory/memory.stat", "total_inactive_file"); ok && inactive < used {
			used -= inactive
		}
	}
	return used, limit
}

func readUint(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return v, err == nil // "max" (no limit) does not parse and reads as unset
}

func statValue(path, key string) (uint64, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) == 2 && f[0] == key {
			v, err := strconv.ParseUint(f[1], 10, 64)
			return v, err == nil
		}
	}
	return 0, false
}

// HostTotal is the machine's RAM (what /proc/meminfo shows inside a container).
func HostTotal() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) >= 2 && f[0] == "MemTotal:" {
			kb, _ := strconv.ParseUint(f[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

// Store keeps each service's latest sample.
type Store interface {
	Upsert(ctx context.Context, service string, s Sample) error
}

// Publish samples this process every interval until ctx ends, so the other
// services (and the web app) can see it.
func Publish(ctx context.Context, store Store, service string, interval time.Duration, log *slog.Logger) {
	write := func() {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := store.Upsert(cctx, service, Take()); err != nil && ctx.Err() == nil {
			log.Warn("memory report failed", "service", service, "error", err)
		}
	}
	write()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			write()
		}
	}
}

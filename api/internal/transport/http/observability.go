package httptransport

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RequestID assigns every request an ID, returned in X-Request-ID and added
// to logs. An incoming X-Request-ID is kept so IDs can be followed across
// proxies, but only if it looks like an ID.
func RequestID() fiber.Handler {
	return requestid.New(requestid.Config{
		Next: func(c fiber.Ctx) bool {
			id := c.Get(fiber.HeaderXRequestID)
			if id != "" && !validRequestID(id) {
				c.Request().Header.Del(fiber.HeaderXRequestID)
			}
			return false
		},
	})
}

func validRequestID(id string) bool {
	if len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// quietPaths are probed constantly and not logged.
var quietPaths = map[string]bool{"/healthz": true, "/readyz": true, "/metrics": true}

// Observe logs each request as a structured line and records metrics. It runs
// the error handler itself so the logged status is the one the client sees.
func Observe(logger *slog.Logger, metrics *Metrics) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		metrics.inFlight.Add(1)
		defer metrics.inFlight.Add(-1)

		if err := c.Next(); err != nil {
			if herr := c.App().Config().ErrorHandler(c, err); herr != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		status := c.Response().StatusCode()
		route := routeLabel(c)
		elapsed := time.Since(start)
		metrics.observe(c.Method(), route, status, elapsed)

		if quietPaths[c.Path()] {
			return nil
		}
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		}
		logger.LogAttrs(c.Context(), level, "request",
			slog.String("request_id", requestid.FromContext(c)),
			slog.String("method", c.Method()),
			slog.String("route", route),
			slog.Int("status", status),
			slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
		)
		return nil
	}
}

// routeLabel returns the route template (such as /api/v1/jobs/:jobID) so logs
// and metrics do not contain IDs and metric cardinality stays bounded. A
// request stopped by group middleware (for example a 401) is labelled with the
// group prefix; one that matched nothing is "unmatched".
func routeLabel(c fiber.Ctx) string {
	r := c.Route()
	if r == nil || r.Path == "" || (!c.Matched() && (r.Path == "/" || strings.Contains(r.Path, "*"))) {
		return "unmatched"
	}
	return r.Path
}

var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type requestKey struct {
	method, route string
	status        int
}

type histogram struct {
	counts []uint64 // per bucket, not cumulative
	sum    float64
	count  uint64
}

// Metrics collects request metrics and renders them in the Prometheus text
// format. It has no dependencies; label values are bounded by the route table.
type Metrics struct {
	started  time.Time
	inFlight atomic.Int64

	mu        sync.Mutex
	requests  map[requestKey]uint64
	durations map[string]*histogram
}

func NewMetrics() *Metrics {
	return &Metrics{
		started:   time.Now(),
		requests:  map[requestKey]uint64{},
		durations: map[string]*histogram{},
	}
}

func (m *Metrics) observe(method, route string, status int, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests[requestKey{method, route, status}]++
	h := m.durations[route]
	if h == nil {
		h = &histogram{counts: make([]uint64, len(durationBuckets))}
		m.durations[route] = h
	}
	secs := elapsed.Seconds()
	for i, upper := range durationBuckets {
		if secs <= upper {
			h.counts[i]++
			break
		}
	}
	h.sum += secs
	h.count++
}

// PoolStats is satisfied by *pgxpool.Pool.
type PoolStats interface {
	Stat() *pgxpool.Stat
}

// RegisterMetricsRoute serves /metrics. The API port is not exposed through
// the web proxy; keep it on a private network.
func RegisterMetricsRoute(app *fiber.App, m *Metrics, pool PoolStats) {
	app.Get("/metrics", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
		return c.SendString(m.render(pool))
	})
}

func (m *Metrics) render(pool PoolStats) string {
	var b strings.Builder
	write := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	write("# HELP rolescout_process_start_time_seconds Start time of the API process.\n")
	write("# TYPE rolescout_process_start_time_seconds gauge\n")
	write("rolescout_process_start_time_seconds %d\n", m.started.Unix())

	write("# HELP rolescout_http_requests_in_flight Requests currently being served.\n")
	write("# TYPE rolescout_http_requests_in_flight gauge\n")
	write("rolescout_http_requests_in_flight %d\n", m.inFlight.Load())

	m.mu.Lock()
	keys := make([]requestKey, 0, len(m.requests))
	for k := range m.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.route != b.route {
			return a.route < b.route
		}
		if a.method != b.method {
			return a.method < b.method
		}
		return a.status < b.status
	})
	write("# HELP rolescout_http_requests_total HTTP requests by method, route and status.\n")
	write("# TYPE rolescout_http_requests_total counter\n")
	for _, k := range keys {
		write("rolescout_http_requests_total{method=%q,route=%q,status=\"%d\"} %d\n", k.method, k.route, k.status, m.requests[k])
	}

	routes := make([]string, 0, len(m.durations))
	for r := range m.durations {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	write("# HELP rolescout_http_request_duration_seconds HTTP request latency by route.\n")
	write("# TYPE rolescout_http_request_duration_seconds histogram\n")
	for _, r := range routes {
		h := m.durations[r]
		var cumulative uint64
		for i, upper := range durationBuckets {
			cumulative += h.counts[i]
			write("rolescout_http_request_duration_seconds_bucket{route=%q,le=%q} %d\n", r, strconv.FormatFloat(upper, 'f', -1, 64), cumulative)
		}
		write("rolescout_http_request_duration_seconds_bucket{route=%q,le=\"+Inf\"} %d\n", r, h.count)
		write("rolescout_http_request_duration_seconds_sum{route=%q} %g\n", r, h.sum)
		write("rolescout_http_request_duration_seconds_count{route=%q} %d\n", r, h.count)
	}
	m.mu.Unlock()

	if pool != nil {
		s := pool.Stat()
		write("# HELP rolescout_db_connections Database pool connections by state.\n")
		write("# TYPE rolescout_db_connections gauge\n")
		write("rolescout_db_connections{state=\"acquired\"} %d\n", s.AcquiredConns())
		write("rolescout_db_connections{state=\"idle\"} %d\n", s.IdleConns())
		write("rolescout_db_connections{state=\"max\"} %d\n", s.MaxConns())
		write("# HELP rolescout_db_acquire_wait_seconds_total Time spent waiting for a pool connection.\n")
		write("# TYPE rolescout_db_acquire_wait_seconds_total counter\n")
		write("rolescout_db_acquire_wait_seconds_total %g\n", s.AcquireDuration().Seconds())
	}
	return b.String()
}

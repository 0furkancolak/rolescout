package research_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/enrich"
	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/llm"
)

// A research run must produce the same result, in about the same time, whether
// the local model is off, or on but dead: the enricher is its own loop and the
// pipeline never waits for it (design addendum, review focus 8).
func TestPipelineIsUnaffectedByADeadModelServer(t *testing.T) {
	f := newPipelineFixture(t)

	// A port nobody listens on.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := l.Addr().String()
	l.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	enricher := &enrich.Enricher{
		Repo:        database.NewEnrichmentRepository(f.pool),
		LLM:         llm.NewClient(llm.Options{BaseURL: "http://" + dead + "/v1", Model: "m", Timeout: time.Second}),
		Model:       "m",
		Concurrency: 1, Batch: 25, MaxChars: 6000, MaxTokens: 100,
		Breaker: enrich.NewBreaker(3, time.Minute, nil),
		Logger:  logger,
	}
	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan struct{})
	go func() { enricher.Loop(ctx, 10*time.Millisecond); close(done) }()
	defer func() { cancel(); <-done }()

	run, err := f.runs.EnqueueRun(f.ctx, f.ws.ID, f.profiles[0].ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	f.p.Tick(f.ctx)
	took := time.Since(start)

	got, err := f.runs.GetRun(f.ctx, f.ws.ID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.RunSucceeded {
		t.Fatalf("run = %+v (error %v)", got, got.Error)
	}
	// The same numbers as TestPipelineEndToEnd, which runs without the enricher.
	if got.Summary["companies_known"] != 3 || got.Summary["jobs_collected"] != 3 || got.Summary["matches"] != 2 || got.Summary["companies_failed"] != 1 {
		t.Errorf("summary with a dead model server = %v", got.Summary)
	}
	if took > 10*time.Second {
		t.Errorf("the run took %s; a dead model server must not slow it", took)
	}
	if !enricher.Breaker.Open() {
		t.Log("note: the enricher did not reach the jobs before the run ended (breaker still closed)")
	}
}

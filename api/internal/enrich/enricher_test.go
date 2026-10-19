package enrich

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/llm"
	"example.com/rolescout/api/internal/llm/llmtest"
)

const goodReply = `{"technologies":["go"],"seniority":"senior","work_arrangement":"remote","remote_policy":"","employment_type":"unknown","salary_min":null,"salary_max":null,"salary_currency":"","salary_period":"unknown","location":"","country_code":"","languages":["en"],"years_experience":null,"summary":"A Go role."}`

type fakeRepo struct {
	mu       sync.Mutex
	queue    []domain.EnrichmentTask
	saved    []domain.EnrichmentResult
	nextArgs []string
}

func (f *fakeRepo) Next(_ context.Context, model string, _, _, limit int, _ time.Time) ([]domain.EnrichmentTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextArgs = append(f.nextArgs, model)
	n := min(limit, len(f.queue))
	out := append([]domain.EnrichmentTask(nil), f.queue[:n]...)
	f.queue = f.queue[n:]
	return out, nil
}

func (f *fakeRepo) Save(_ context.Context, r domain.EnrichmentResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, r)
	return nil
}

func tasks(n int) []domain.EnrichmentTask {
	out := make([]domain.EnrichmentTask, n)
	for i := range out {
		out[i] = domain.EnrichmentTask{JobID: fmt.Sprintf("job-%d", i), Title: "Engineer", CompanyName: "Acme",
			Description: "We use Go and build services. Senior engineers welcome.", DescriptionHash: fmt.Sprintf("hash-%d", i)}
	}
	return out
}

type harness struct {
	repo *fakeRepo
	srv  *llmtest.Server
	e    *Enricher
	clk  *clock
	logs *bytes.Buffer
}

func newHarness(t *testing.T, mutate ...func(*Enricher)) *harness {
	t.Helper()
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: goodReply, PromptTokens: 100, CompletionTokens: 20})
	repo := &fakeRepo{}
	clk := &clock{t: time.Date(2031, 1, 1, 12, 0, 0, 0, time.UTC)}
	logs := &bytes.Buffer{}
	e := &Enricher{
		Repo:        repo,
		LLM:         llm.NewClient(llm.Options{BaseURL: srv.BaseURL(), Model: "primary", Timeout: 2 * time.Second}),
		Model:       "primary",
		Concurrency: 1, Batch: 25, MaxChars: 6000, MaxTokens: 400,
		Breaker: NewBreaker(3, 10*time.Minute, clk.now),
		Now:     clk.now,
		Logger:  slog.New(slog.NewJSONHandler(logs, nil)),
	}
	for _, m := range mutate {
		m(e)
	}
	return &harness{repo: repo, srv: srv, e: e, clk: clk, logs: logs}
}

func TestTickEnrichesABatchInQueueOrder(t *testing.T) {
	h := newHarness(t, func(e *Enricher) { e.Batch = 3 })
	h.repo.queue = tasks(5)
	res, err := h.e.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Enriched != 3 || res.Failed != 0 || len(h.repo.saved) != 3 {
		t.Fatalf("result = %+v, saved = %d", res, len(h.repo.saved))
	}
	for i, s := range h.repo.saved {
		if s.JobID != fmt.Sprintf("job-%d", i) || !s.OK || s.Model != "primary" || s.PromptVersion != llm.PromptVersion || s.SchemaVersion != llm.SchemaVersion ||
			s.DescriptionHash != fmt.Sprintf("hash-%d", i) || s.PromptTokens != 100 || s.CompletionTokens != 20 {
			t.Fatalf("saved[%d] = %+v", i, s)
		}
		var en llm.Enrichment
		if err := json.Unmarshal(s.Data, &en); err != nil || en.Seniority != "senior" || len(en.Technologies) != 1 {
			t.Fatalf("data = %s (%v)", s.Data, err)
		}
	}
	if len(h.repo.queue) != 2 {
		t.Fatalf("queue left = %d, want 2", len(h.repo.queue))
	}
}

func TestConcurrencyNeverExceedsTheLimit(t *testing.T) {
	h := newHarness(t, func(e *Enricher) { e.Concurrency = 2 })
	h.srv.Script(llmtest.Scripted{Content: goodReply, Delay: 60 * time.Millisecond})
	h.repo.queue = tasks(8)
	res, err := h.e.Tick(context.Background())
	if err != nil || res.Enriched != 8 {
		t.Fatalf("result = %+v, %v", res, err)
	}
	if got := h.srv.MaxConcurrent(); got != 2 {
		t.Fatalf("max concurrent model requests = %d, want exactly the limit 2", got)
	}
}

func TestNothingQueuedMakesNoModelCalls(t *testing.T) {
	h := newHarness(t)
	res, err := h.e.Tick(context.Background())
	if err != nil || res.Enriched != 0 || h.srv.Chats() != 0 {
		t.Fatalf("result = %+v err = %v chats = %d", res, err, h.srv.Chats())
	}
}

func TestUnavailableServerOpensTheBreakerOnce(t *testing.T) {
	h := newHarness(t)
	h.srv.Script(llmtest.Scripted{Status: 500})
	h.repo.queue = tasks(10)

	res, _ := h.e.Tick(context.Background())
	if h.srv.Chats() != 3 || res.Enriched != 0 {
		t.Fatalf("chats = %d (want 3, then the breaker opens), result %+v", h.srv.Chats(), res)
	}
	if len(h.repo.saved) != 0 {
		t.Fatalf("an unavailable server must not burn job attempts: saved %d", len(h.repo.saved))
	}
	// During the cool-down nothing is sent and nothing is logged again.
	before := strings.Count(h.logs.String(), "\n")
	for i := 0; i < 3; i++ {
		if _, err := h.e.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if h.srv.Chats() != 3 {
		t.Fatalf("requests during the cool-down: %d", h.srv.Chats()-3)
	}
	if lines := strings.Count(h.logs.String(), "\n") - before; lines > 1 {
		t.Fatalf("%d log lines during the cool-down, want at most one", lines)
	}
	if warns := strings.Count(h.logs.String(), `"level":"WARN"`); warns != 1 {
		t.Fatalf("%d warnings in total, want exactly one", warns)
	}

	// After the cool-down a probe goes out; when the server is back it recovers.
	h.srv.Script(llmtest.Scripted{Content: goodReply})
	h.clk.advance(11 * time.Minute)
	h.repo.queue = tasks(2) // the real queue still holds the jobs that were not tried
	res, err := h.e.Tick(context.Background())
	if err != nil || res.Enriched == 0 || h.e.Breaker.Open() {
		t.Fatalf("recovery: result %+v err %v open=%v", res, err, h.e.Breaker.Open())
	}
}

func TestFallbackModelWhileDegraded(t *testing.T) {
	h := newHarness(t, func(e *Enricher) { e.FallbackModel = "tiny" })
	h.srv.Script(llmtest.Scripted{Status: 500}, llmtest.Scripted{Status: 500}, llmtest.Scripted{Status: 500}, llmtest.Scripted{Content: goodReply})
	h.repo.queue = tasks(3)
	h.e.Tick(context.Background()) // three failures open the breaker
	h.repo.queue = tasks(2)
	res, err := h.e.Tick(context.Background())
	if err != nil || res.Enriched != 2 || !res.Degraded {
		t.Fatalf("degraded tick: %+v, %v", res, err)
	}
	last := h.repo.nextArgs[len(h.repo.nextArgs)-1]
	if last != "tiny" {
		t.Fatalf("queue asked for model %q, want the fallback", last)
	}
	for _, s := range h.repo.saved {
		if s.Model != "tiny" {
			t.Fatalf("saved under model %q, want tiny", s.Model)
		}
	}
	if !strings.Contains(string(h.srv.Requests()[3].Body), `"model":"tiny"`) {
		t.Fatalf("request model: %s", h.srv.Requests()[3].Body)
	}
}

func TestInvalidOutputIsSavedAsFailedWithoutTrippingTheBreaker(t *testing.T) {
	h := newHarness(t)
	h.srv.Script(llmtest.Scripted{Content: "garbage"}, llmtest.Scripted{Content: "more garbage"})
	h.repo.queue = tasks(1)
	res, err := h.e.Tick(context.Background())
	if err != nil || res.Failed != 1 || res.Enriched != 0 {
		t.Fatalf("result = %+v, %v", res, err)
	}
	s := h.repo.saved[0]
	if s.OK || s.Error != "invalid output" || len(s.Data) != 0 {
		t.Fatalf("saved = %+v", s)
	}
	if h.e.Breaker.Open() {
		t.Fatal("bad output is the model's fault, not the server's")
	}
}

func TestTimeoutsAreSavedAsFailedAndCountTowardsTheBreaker(t *testing.T) {
	h := newHarness(t)
	h.e.LLM = llm.NewClient(llm.Options{BaseURL: h.srv.BaseURL(), Model: "primary", Timeout: 80 * time.Millisecond})
	h.srv.Script(llmtest.Scripted{Content: goodReply, Delay: time.Second})
	h.repo.queue = tasks(5)
	h.e.Tick(context.Background())
	if len(h.repo.saved) != 3 || h.repo.saved[0].OK || h.repo.saved[0].Error != "timeout" {
		t.Fatalf("saved = %+v", h.repo.saved)
	}
	if !h.e.Breaker.Open() {
		t.Fatal("three timeouts in a row must open the breaker")
	}
}

func TestCancellationReturnsPromptly(t *testing.T) {
	h := newHarness(t, func(e *Enricher) { e.Concurrency = 2 })
	h.srv.Script(llmtest.Scripted{Content: goodReply, Delay: 5 * time.Second})
	h.repo.queue = tasks(6)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.e.Tick(ctx); close(done) }()
	time.Sleep(150 * time.Millisecond)
	start := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Tick did not return within a second of cancellation")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("took %s", time.Since(start))
	}
	if len(h.repo.saved) != 0 {
		t.Fatalf("cancelled requests must not be stored as failures: %+v", h.repo.saved)
	}
}

func TestLoopStopsOnCancel(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { h.e.Loop(ctx, 20*time.Millisecond); close(done) }()
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Loop did not stop")
	}
}

type fakeExplainRepo struct {
	fakeRepo
	explainQueue []domain.ExplanationTask
	explained    []string
	gotModel     string
	gotMin       int
}

func (f *fakeExplainRepo) ExplanationQueue(_ context.Context, model string, _ int, minScore, limit int) ([]domain.ExplanationTask, error) {
	f.gotModel, f.gotMin = model, minScore
	n := min(limit, len(f.explainQueue))
	out := append([]domain.ExplanationTask(nil), f.explainQueue[:n]...)
	f.explainQueue = f.explainQueue[n:]
	return out, nil
}

func (f *fakeExplainRepo) SaveExplanation(_ context.Context, ws, profile, job, hash, model string, _ int, text string) error {
	f.explained = append(f.explained, job+"|"+ws+"|"+profile+"|"+hash+"|"+model+"|"+text)
	return nil
}

func explainTasks(n int) []domain.ExplanationTask {
	out := make([]domain.ExplanationTask, n)
	for i := range out {
		out[i] = domain.ExplanationTask{WorkspaceID: "ws", ProfileID: "p", JobID: fmt.Sprintf("job-%d", i), DescriptionHash: "h", Title: "Engineer", CompanyName: "Acme",
			Score: 80, Technologies: []string{"go"}, MatchedTechnologies: []string{"go"}, Locale: "en", Reasons: []domain.MatchReason{{Code: "remote"}}}
	}
	return out
}

func TestExplanationsFollowEnrichmentForStrongMatches(t *testing.T) {
	h := newHarness(t, func(e *Enricher) { e.Batch = 2; e.ExplainMinScore = 70 })
	repo := &fakeExplainRepo{explainQueue: explainTasks(5)}
	h.e.Repo = repo
	h.srv.Script(llmtest.Scripted{Content: `{"explanation":"A remote Go role that fits your search."}`})
	res, err := h.e.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Explained != 2 || len(repo.explained) != 2 || repo.gotMin != 70 || repo.gotModel != "primary" {
		t.Fatalf("result %+v explained %v min %d model %q", res, repo.explained, repo.gotMin, repo.gotModel)
	}
	if !strings.Contains(repo.explained[0], "job-0|ws|p|h|primary|A remote Go role") {
		t.Fatalf("explained[0] = %s", repo.explained[0])
	}
}

func TestExplanationsAreSkippedWhileDegradedAndOnInvalidText(t *testing.T) {
	h := newHarness(t, func(e *Enricher) { e.FallbackModel = "tiny"; e.ExplainMinScore = 70 })
	repo := &fakeExplainRepo{explainQueue: explainTasks(2)}
	h.e.Repo = repo
	for i := 0; i < 3; i++ {
		h.e.Breaker.Failure()
	}
	h.e.Tick(context.Background())
	if len(repo.explained) != 0 || h.srv.Chats() != 0 {
		t.Fatalf("degraded mode must not write explanations (chats %d, explained %v)", h.srv.Chats(), repo.explained)
	}

	h2 := newHarness(t, func(e *Enricher) { e.ExplainMinScore = 70 })
	repo2 := &fakeExplainRepo{explainQueue: explainTasks(1)}
	h2.e.Repo = repo2
	h2.srv.Script(llmtest.Scripted{Content: `{"explanation":"It uses Rust and 400 engineers."}`})
	res, _ := h2.e.Tick(context.Background())
	// The attempt is recorded with an empty text, so the same match is not tried
	// again on every tick; nothing readable is stored.
	if res.Explained != 0 || len(repo2.explained) != 1 || !strings.HasSuffix(repo2.explained[0], "|") {
		t.Fatalf("an ungrounded explanation was stored: %+v %v", res, repo2.explained)
	}
}

func TestMissingModelKeepsJobsQueuedAndOpensTheBreaker(t *testing.T) {
	h := newHarness(t)
	h.srv.Script(llmtest.Scripted{Status: 404})
	h.repo.queue = tasks(10)
	h.e.Tick(context.Background())
	if len(h.repo.saved) != 0 || !h.e.Breaker.Open() {
		t.Fatalf("404 must not burn job attempts and must open the breaker: saved %d open %v", len(h.repo.saved), h.e.Breaker.Open())
	}
}

func TestHeldExplanationsWaitAndGrow(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	e := &Enricher{Now: func() time.Time { return now }}
	if e.isHeld("k") {
		t.Fatal("nothing is held yet")
	}
	e.hold("k")
	if !e.isHeld("k") || e.heldCount() != 1 {
		t.Fatal("a failed explanation must wait")
	}
	now = now.Add(5*time.Minute + time.Second)
	if e.isHeld("k") {
		t.Fatal("the first wait is 5 minutes")
	}
	e.hold("k") // fails again: 10 minutes
	now = now.Add(6 * time.Minute)
	if !e.isHeld("k") {
		t.Fatal("the second wait doubles")
	}
	now = now.Add(5 * time.Minute)
	if e.isHeld("k") {
		t.Fatal("the second wait is 10 minutes")
	}
	e.release("k")
	if e.heldCount() != 0 {
		t.Fatal("a success forgets the wait")
	}
}

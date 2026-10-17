package research

import (
	"context"
	"maps"
	"sync"
	"time"

	"example.com/rolescout/api/internal/domain"
)

// reporter collects a run's stage progress and log lines and writes them in
// batches. Each flush also checks whether the run was cancelled.
type reporter struct {
	runs   RunStore
	cancel context.CancelFunc
	now    func() time.Time

	mu        sync.Mutex
	run       domain.ResearchRun
	events    []domain.RunEvent
	cancelled bool
}

func newReporter(runs RunStore, run domain.ResearchRun, cancel context.CancelFunc, now func() time.Time) *reporter {
	if run.Stages == nil {
		run.Stages = map[domain.Stage]domain.StageProgress{}
	}
	for _, s := range domain.Stages {
		if _, ok := run.Stages[s]; !ok {
			run.Stages[s] = domain.StageProgress{State: "pending"}
		}
	}
	return &reporter{runs: runs, run: run, cancel: cancel, now: now}
}

// start flushes every interval until the returned stop function is called;
// stop flushes once more.
func (r *reporter) start(ctx context.Context, interval time.Duration) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				r.flush(ctx)
			}
		}
	}()
	return func() {
		close(done)
		<-finished
		r.flush(ctx)
	}
}

func (r *reporter) flush(ctx context.Context) {
	r.mu.Lock()
	run := r.run
	run.Stages = make(map[domain.Stage]domain.StageProgress, len(r.run.Stages))
	for k, v := range r.run.Stages {
		// Detail is a map that update keeps changing; save a copy.
		v.Detail = maps.Clone(v.Detail)
		run.Stages[k] = v
	}
	events := r.events
	r.events = nil
	r.mu.Unlock()

	_ = r.runs.SaveProgress(ctx, run)
	_ = r.runs.AddEvents(ctx, events)
	if c, err := r.runs.CancelRequested(ctx, run.ID); err == nil && c {
		r.mu.Lock()
		r.cancelled = true
		r.mu.Unlock()
		r.cancel()
	}
}

func (r *reporter) isCancelled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cancelled
}

func (r *reporter) update(stage domain.Stage, fn func(*domain.StageProgress)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.run.Stages[stage]
	fn(&p)
	r.run.Stages[stage] = p
}

func (r *reporter) begin(stage domain.Stage) {
	t := r.now()
	r.mu.Lock()
	r.run.Stage = stage
	r.mu.Unlock()
	r.update(stage, func(p *domain.StageProgress) {
		p.State, p.StartedAt = "running", &t
		if p.Detail == nil {
			p.Detail = map[string]int{}
		}
	})
}

func (r *reporter) end(stage domain.Stage, state, note string) {
	t := r.now()
	r.update(stage, func(p *domain.StageProgress) {
		p.State, p.FinishedAt = state, &t
		if note != "" {
			p.Note = note
		}
	})
}

func (r *reporter) stage(s domain.Stage) domain.StageProgress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.run.Stages[s]
}

func (r *reporter) event(level string, stage domain.Stage, company, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, domain.RunEvent{
		RunID: r.run.ID, At: r.now(), Level: level, Stage: stage, Company: company, Message: message,
	})
}

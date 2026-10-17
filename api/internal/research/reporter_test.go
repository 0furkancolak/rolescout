package research

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

// jsonRuns marshals saved progress like the database does.
type jsonRuns struct{ fakeRuns }

func (j *jsonRuns) SaveProgress(_ context.Context, run domain.ResearchRun) error {
	_, err := json.Marshal(run.Stages)
	return err
}

func TestReporterFlushDoesNotShareDetailMaps(t *testing.T) {
	runs := &jsonRuns{fakeRuns: *newFakeRuns()}
	rep := newReporter(runs, testRun, func() {}, time.Now)
	rep.begin(domain.StageDetection)
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			rep.update(domain.StageDetection, func(sp *domain.StageProgress) { sp.Detail["lever"]++ })
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			rep.flush(ctx)
		}
	}()
	wg.Wait()
	if got := rep.stage(domain.StageDetection).Detail["lever"]; got != 2000 {
		t.Fatalf("detail = %d", got)
	}
}

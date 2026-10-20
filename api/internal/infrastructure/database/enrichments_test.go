package database

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"example.com/rolescout/api/internal/domain"
)

const (
	testModel  = "test-model"
	testPrompt = 1
	testSchema = 1
)

type enrichFixture struct {
	fixture
	repo *EnrichmentRepository
}

func newEnrichFixture(t *testing.T) enrichFixture {
	t.Helper()
	f := newFixture(t)
	return enrichFixture{fixture: f, repo: NewEnrichmentRepository(requireDB(t))}
}

// job creates an open job with the given description.
func (ef enrichFixture) job(t *testing.T, title, description string) domain.Job {
	t.Helper()
	ctx := context.Background()
	j, _, err := NewJobRepository(requireDB(t)).UpsertListing(ctx, IngestedListing{
		SourceID: ef.source.ID, ExternalID: uniq("e"), URL: "https://x.example/" + uniq("u"),
		DedupeKey: uniq("dk"), Title: title, CompanyName: "Acme", Description: description, SeenAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func (ef enrichFixture) next(t *testing.T, now time.Time) []domain.EnrichmentTask {
	t.Helper()
	tasks, err := ef.repo.Next(context.Background(), testModel, testPrompt, testSchema, 100000, now)
	if err != nil {
		t.Fatal(err)
	}
	return tasks
}

// order returns the positions of the given job IDs in the queue, in queue order.
func queueOrder(tasks []domain.EnrichmentTask, ids ...string) []string {
	var out []string
	for _, tk := range tasks {
		if slices.Contains(ids, tk.JobID) {
			out = append(out, tk.JobID)
		}
	}
	return out
}

func (ef enrichFixture) saveOK(t *testing.T, tk domain.EnrichmentTask, data string) {
	t.Helper()
	err := ef.repo.Save(context.Background(), domain.EnrichmentResult{
		JobID: tk.JobID, DescriptionHash: tk.DescriptionHash, Model: testModel, PromptVersion: testPrompt, SchemaVersion: testSchema,
		OK: true, Data: json.RawMessage(data), PromptTokens: 100, CompletionTokens: 20, Latency: 1500 * time.Millisecond, Now: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (ef enrichFixture) saveFailed(t *testing.T, tk domain.EnrichmentTask, now time.Time) {
	t.Helper()
	err := ef.repo.Save(context.Background(), domain.EnrichmentResult{
		JobID: tk.JobID, DescriptionHash: tk.DescriptionHash, Model: testModel, PromptVersion: testPrompt, SchemaVersion: testSchema,
		OK: false, Error: "invalid output", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func taskFor(t *testing.T, tasks []domain.EnrichmentTask, jobID string) domain.EnrichmentTask {
	t.Helper()
	for _, tk := range tasks {
		if tk.JobID == jobID {
			return tk
		}
	}
	t.Fatalf("job %s is not in the queue", jobID)
	return domain.EnrichmentTask{}
}

func TestQueueOrdersMatchesThenSavedThenNewest(t *testing.T) {
	ef := newEnrichFixture(t)
	pool := requireDB(t)
	ctx := context.Background()
	research := NewResearchRepository(pool)

	oldest := ef.job(t, "Oldest unmatched", "text about "+uniq("a"))
	newest := ef.job(t, "Newest unmatched", "text about "+uniq("b"))
	saved := ef.job(t, "Saved", "text about "+uniq("c"))
	low := ef.job(t, "Low match", "text about "+uniq("d"))
	high := ef.job(t, "High match", "text about "+uniq("e"))
	pool.Exec(ctx, `UPDATE jobs SET first_seen_at = NOW() - INTERVAL '30 days' WHERE id = $1`, oldest.ID)
	profile, err := research.CreateProfile(ctx, ef.workspace.ID, profileParams(uniq("q")))
	if err != nil {
		t.Fatal(err)
	}
	for id, score := range map[string]int{low.ID: 50, high.ID: 90} {
		if _, err := pool.Exec(ctx, `INSERT INTO research_matches (profile_id, job_id, score) VALUES ($1, $2, $3)`, profile.ID, id, score); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO saved_jobs (workspace_id, job_id) VALUES ($1, $2)`, ef.workspace.ID, saved.ID); err != nil {
		t.Fatal(err)
	}

	got := queueOrder(ef.next(t, time.Now()), oldest.ID, newest.ID, saved.ID, low.ID, high.ID)
	want := []string{high.ID, low.ID, saved.ID, newest.ID, oldest.ID}
	if !slices.Equal(got, want) {
		t.Fatalf("queue order = %v, want %v", got, want)
	}
}

func TestQueueSkipsClosedAndEmptyJobs(t *testing.T) {
	ef := newEnrichFixture(t)
	ctx := context.Background()
	closed := ef.job(t, "Closed", "text "+uniq("c"))
	empty := ef.job(t, "Empty description", "   ")
	open := ef.job(t, "Open", "text "+uniq("o"))
	requireDB(t).Exec(ctx, `UPDATE jobs SET closed_at = NOW() WHERE id = $1`, closed.ID)
	got := queueOrder(ef.next(t, time.Now()), closed.ID, empty.ID, open.ID)
	if !slices.Equal(got, []string{open.ID}) {
		t.Fatalf("queue = %v, want only the open job with text", got)
	}
}

func TestSameDescriptionIsReadOnce(t *testing.T) {
	ef := newEnrichFixture(t)
	text := "Shared posting text " + uniq("s")
	a := ef.job(t, "Engineer", text)
	b := ef.job(t, "Engineer (copy)", "  "+text+"\n\n ") // whitespace differs only
	tasks := ef.next(t, time.Now())
	if len(queueOrder(tasks, a.ID, b.ID)) != 2 {
		t.Fatal("both jobs should be queued before anything is saved")
	}
	ef.saveOK(t, taskFor(t, tasks, a.ID), `{"seniority":"senior"}`)

	// The second job is not queued: its result is copied from the first.
	again := ef.next(t, time.Now())
	if got := queueOrder(again, a.ID, b.ID); len(got) != 0 {
		t.Fatalf("queue after save = %v, want none (cached by description hash)", got)
	}
	latest, err := ef.repo.Latest(context.Background(), []string{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if string(latest[b.ID].Data) != `{"seniority": "senior"}` && string(latest[b.ID].Data) != `{"seniority":"senior"}` {
		t.Fatalf("copied data = %s", latest[b.ID].Data)
	}
	if latest[a.ID].Model != testModel || latest[b.ID].JobID != b.ID {
		t.Fatalf("latest = %+v", latest)
	}
}

func TestChangedDescriptionRequeues(t *testing.T) {
	ef := newEnrichFixture(t)
	j := ef.job(t, "Engineer", "first text "+uniq("c"))
	tasks := ef.next(t, time.Now())
	ef.saveOK(t, taskFor(t, tasks, j.ID), `{"seniority":"unknown"}`)
	if len(queueOrder(ef.next(t, time.Now()), j.ID)) != 0 {
		t.Fatal("an enriched job must not be queued again")
	}
	if _, err := requireDB(t).Exec(context.Background(), `UPDATE jobs SET description = $2 WHERE id = $1`, j.ID, "completely new text "+uniq("n")); err != nil {
		t.Fatal(err)
	}
	if len(queueOrder(ef.next(t, time.Now()), j.ID)) != 1 {
		t.Fatal("a changed description must be queued again")
	}
}

func TestNewPromptVersionRequeuesAtLowestPriority(t *testing.T) {
	ef := newEnrichFixture(t)
	old := ef.job(t, "Already enriched", "old text "+uniq("o"))
	fresh := ef.job(t, "Never enriched", "fresh text "+uniq("f"))
	tasks := ef.next(t, time.Now())
	ef.saveOK(t, taskFor(t, tasks, old.ID), `{}`)
	// Make the already-enriched job look far more important than the fresh one.
	pool := requireDB(t)
	ctx := context.Background()
	profile, _ := NewResearchRepository(pool).CreateProfile(ctx, ef.workspace.ID, profileParams(uniq("p")))
	pool.Exec(ctx, `INSERT INTO research_matches (profile_id, job_id, score) VALUES ($1, $2, 99)`, profile.ID, old.ID)

	got, err := ef.repo.Next(ctx, testModel, testPrompt+1, testSchema, 100000, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	order := queueOrder(got, old.ID, fresh.ID)
	if !slices.Equal(order, []string{fresh.ID, old.ID}) {
		t.Fatalf("order under the new prompt = %v, want never-enriched first then the re-queued job", order)
	}
}

func TestFailureBackoffAndGiveUp(t *testing.T) {
	ef := newEnrichFixture(t)
	j := ef.job(t, "Hard one", "text that fails "+uniq("h"))
	pool := requireDB(t)
	ctx := context.Background()
	t0 := time.Date(2031, 5, 1, 12, 0, 0, 0, time.UTC)

	wait := func(n int) time.Duration {
		// 1 h doubling per attempt, capped at 24 h.
		d := time.Hour << (n - 1)
		return min(d, 24*time.Hour)
	}
	now := t0
	for attempt := 1; attempt <= 5; attempt++ {
		tk := taskFor(t, ef.next(t, now), j.ID)
		ef.saveFailed(t, tk, now)
		var next time.Time
		var attempts int
		if err := pool.QueryRow(ctx, `SELECT next_attempt_at, attempts FROM job_enrichments WHERE job_id = $1`, j.ID).Scan(&next, &attempts); err != nil {
			t.Fatal(err)
		}
		if attempts != attempt || !next.Equal(now.Add(wait(attempt))) {
			t.Fatalf("attempt %d: attempts=%d next=%s, want %s", attempt, attempts, next, now.Add(wait(attempt)))
		}
		if attempt < 5 {
			if len(queueOrder(ef.next(t, now.Add(wait(attempt)-time.Minute)), j.ID)) != 0 {
				t.Fatalf("attempt %d: queued before the backoff ended", attempt)
			}
			now = now.Add(wait(attempt) + time.Minute)
		}
	}
	// After five attempts it stays failed.
	if len(queueOrder(ef.next(t, now.Add(1000*time.Hour)), j.ID)) != 0 {
		t.Fatal("a job that failed five times must not be queued again")
	}
	// A new prompt version gives it another chance.
	got, _ := ef.repo.Next(ctx, testModel, testPrompt+1, testSchema, 100000, now)
	if len(queueOrder(got, j.ID)) != 1 {
		t.Fatal("a new prompt version must retry failed jobs")
	}
}

func TestEnrichmentCascadesWithJob(t *testing.T) {
	ef := newEnrichFixture(t)
	j := ef.job(t, "Doomed", "text "+uniq("d"))
	ef.saveOK(t, taskFor(t, ef.next(t, time.Now()), j.ID), `{}`)
	pool := requireDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM jobs WHERE id = $1`, j.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	pool.QueryRow(ctx, `SELECT count(*) FROM job_enrichments WHERE job_id = $1`, j.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d enrichment rows left after the job was deleted", n)
	}
}

func TestStats(t *testing.T) {
	ef := newEnrichFixture(t)
	ctx := context.Background()
	model := uniq("model-")
	a := ef.job(t, "One", "stats text "+uniq("a"))
	b := ef.job(t, "Two", "stats text "+uniq("b"))
	tasks, err := ef.repo.Next(ctx, model, 1, 1, 100000, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	save := func(tk domain.EnrichmentTask, ok bool, ms int) {
		r := domain.EnrichmentResult{JobID: tk.JobID, DescriptionHash: tk.DescriptionHash, Model: model, PromptVersion: 1, SchemaVersion: 1,
			OK: ok, Latency: time.Duration(ms) * time.Millisecond, Now: time.Now()}
		if ok {
			r.Data = json.RawMessage(`{}`)
		} else {
			r.Error = "timeout"
		}
		if err := ef.repo.Save(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	save(taskFor(t, tasks, a.ID), true, 2000)
	save(taskFor(t, tasks, b.ID), false, 0)
	st, err := ef.repo.Stats(ctx, model, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if st.Enriched != 1 || st.Failed != 1 || st.AvgLatencyMs != 2000 || st.LastError != "timeout" {
		t.Fatalf("stats = %+v", st)
	}
	// A result of an older prompt neither counts nor takes the job out of the queue.
	if old, err := ef.repo.Stats(ctx, model, 2, 1); err != nil || old.Enriched != 0 || old.Queue < st.Queue+1 {
		t.Fatalf("stats for a newer prompt = %+v, %v (queue was %d)", old, err, st.Queue)
	}
}

func TestExplanationsAreWorkspacePrivate(t *testing.T) {
	ef := newEnrichFixture(t)
	other := newFixture(t)
	pool := requireDB(t)
	ctx := context.Background()
	research := NewResearchRepository(pool)
	j := ef.job(t, "Match", "explain text "+uniq("e"))
	p1, _ := research.CreateProfile(ctx, ef.workspace.ID, profileParams(uniq("p")))
	p2, _ := research.CreateProfile(ctx, other.workspace.ID, profileParams(uniq("p")))
	var hash string
	pool.QueryRow(ctx, `SELECT description_hash FROM jobs WHERE id = $1`, j.ID).Scan(&hash)

	if err := ef.repo.SaveExplanation(ctx, ef.workspace.ID, p1.ID, j.ID, hash, testModel, testPrompt, "Matches because of Go."); err != nil {
		t.Fatal(err)
	}
	got, err := ef.repo.Explanations(ctx, ef.workspace.ID, p1.ID, []string{j.ID})
	if err != nil || got[j.ID] != "Matches because of Go." {
		t.Fatalf("own explanation = %v, %v", got, err)
	}
	if mine, _ := ef.repo.Explanations(ctx, other.workspace.ID, p2.ID, []string{j.ID}); len(mine) != 0 {
		t.Fatalf("another workspace sees %v", mine)
	}
	// Mixing a workspace with another workspace's profile finds nothing.
	if cross, _ := ef.repo.Explanations(ctx, other.workspace.ID, p1.ID, []string{j.ID}); len(cross) != 0 {
		t.Fatalf("cross-workspace profile returned %v", cross)
	}
	// A changed description makes the explanation stale.
	pool.Exec(ctx, `UPDATE jobs SET description = 'something else entirely' WHERE id = $1`, j.ID)
	if stale, _ := ef.repo.Explanations(ctx, ef.workspace.ID, p1.ID, []string{j.ID}); len(stale) != 0 {
		t.Fatalf("stale explanation still returned: %v", stale)
	}
}

// effectiveJob creates an open job with the given rule values.
func (ef enrichFixture) effectiveJob(t *testing.T, title string, seniority, arrangement string, technologies []string, country *string) domain.Job {
	t.Helper()
	j := ef.job(t, title, "description for "+title+" "+uniq("d"))
	_, err := requireDB(t).Exec(context.Background(),
		`UPDATE jobs SET seniority = $2, work_arrangement = $3, technologies = $4, country_code = $5 WHERE id = $1`,
		j.ID, seniority, arrangement, technologies, country)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func (ef enrichFixture) enrich(t *testing.T, j domain.Job, data string) {
	t.Helper()
	var hash string
	requireDB(t).QueryRow(context.Background(), `SELECT description_hash FROM jobs WHERE id = $1`, j.ID).Scan(&hash)
	ef.saveOK(t, domain.EnrichmentTask{JobID: j.ID, DescriptionHash: hash}, data)
}

func TestEffectiveFieldsRulesWinAndModelFillsGaps(t *testing.T) {
	ef := newEnrichFixture(t)
	jobs := NewJobRepository(requireDB(t))
	ctx := context.Background()
	tr := "TR"

	rulesWin := ef.effectiveJob(t, "A rules win", "senior", "remote", []string{"go"}, &tr)
	gapFilled := ef.effectiveJob(t, "B gap filled", "unknown", "unknown", []string{"go"}, nil)
	_ = ef.effectiveJob(t, "C no model", "unknown", "unknown", []string{"go"}, nil)
	stale := ef.effectiveJob(t, "D stale model", "unknown", "unknown", []string{"go"}, nil)
	ai := `{"seniority":"junior","work_arrangement":"hybrid","country_code":"DE","technologies":["go","rust"],"summary":"x"}`
	ef.enrich(t, rulesWin, ai)
	ef.enrich(t, gapFilled, ai)
	ef.enrich(t, stale, ai)
	requireDB(t).Exec(ctx, `UPDATE jobs SET description = 'rewritten after the model read it' WHERE id = $1`, stale.ID)

	byTitle := func(filter JobFilter) map[string]bool {
		filter.Limit = 100000
		list, _, err := jobs.WorkspaceJobs(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]domain.Job{}
		got := map[string]bool{}
		for _, wj := range list {
			out[wj.Job.Title] = wj.Job
			got[wj.Job.Title] = true
		}
		// The jobs themselves carry the effective values.
		if j, ok := out["A rules win"]; ok && (j.Seniority != "senior" || j.WorkArrangement != "remote" || *j.CountryCode != "TR") {
			t.Fatalf("rules did not win: %+v", j)
		}
		if j, ok := out["B gap filled"]; ok && (j.Seniority != "junior" || j.WorkArrangement != "hybrid" || *j.CountryCode != "DE" || len(j.Technologies) != 2) {
			t.Fatalf("gap not filled: %+v", j)
		}
		if j, ok := out["C no model"]; ok && (j.Seniority != "unknown" || j.CountryCode != nil || len(j.Technologies) != 1) {
			t.Fatalf("untouched job changed: %+v", j)
		}
		return got
	}
	mine := func(got map[string]bool) []string {
		var out []string
		for _, k := range []string{"A rules win", "B gap filled", "C no model", "D stale model"} {
			if got[k] {
				out = append(out, k)
			}
		}
		return out
	}
	base := JobFilter{WorkspaceID: ef.workspace.ID}
	check := func(name string, f JobFilter, want ...string) {
		t.Helper()
		if got := mine(byTitle(f)); !slices.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	senior := base
	senior.Seniority = "senior"
	check("seniority=senior (rules)", senior, "A rules win")
	junior := base
	junior.Seniority = "junior"
	check("seniority=junior (model fills the gap, rules still win for A)", junior, "B gap filled")
	hybrid := base
	hybrid.WorkArrangement = "hybrid"
	check("arrangement=hybrid", hybrid, "B gap filled")
	de := base
	de.Countries = []string{"DE"}
	check("country=DE", de, "B gap filled")
	rust := base
	rust.Technologies = []string{"rust"}
	check("technology=rust (union with rules)", rust, "A rules win", "B gap filled")
	golang := base
	golang.Technologies = []string{"go"}
	check("technology=go", golang, "A rules win", "B gap filled", "C no model", "D stale model")

	// Facets use the effective values, and a technology present in both the
	// rules and the model result is counted once per job.
	fc, err := jobs.Facets(ctx, JobFilter{WorkspaceID: ef.workspace.ID, Technologies: nil, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, c := range fc.Technologies {
		counts[c.Value] = c.Count
	}
	if counts["go"] < 4 || counts["rust"] < 2 {
		t.Errorf("technology facet = %v", counts)
	}
	for _, c := range fc.Technologies {
		if c.Value == "go" && c.Count > 4+100 {
			t.Errorf("go counted %d times", c.Count)
		}
	}
	if fc.Seniority["junior"] < 1 || fc.Arrangement["hybrid"] < 1 || fc.Country["DE"] < 1 {
		t.Errorf("facets ignore the model: %+v", fc)
	}
}

func TestOpenJobsForScoringSeesEffectiveFields(t *testing.T) {
	ef := newEnrichFixture(t)
	ctx := context.Background()
	jobs := NewJobRepository(requireDB(t))
	with := ef.effectiveJob(t, "Scored with model", "unknown", "unknown", []string{"go"}, nil)
	without := ef.effectiveJob(t, "Scored without model", "unknown", "unknown", []string{"go"}, nil)
	ef.enrich(t, with, `{"seniority":"senior","work_arrangement":"remote","country_code":"","technologies":["go","react"]}`)

	got := map[string]domain.Job{}
	if err := jobs.OpenJobsForScoring(ctx, ef.workspace.ID, func(j domain.Job) error {
		got[j.ID] = j
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	w, wo := got[with.ID], got[without.ID]
	if !slices.Equal(w.Technologies, []string{"go", "react"}) || w.Seniority != "senior" || w.WorkArrangement != "remote" {
		t.Errorf("with model: %+v", w)
	}
	if !slices.Equal(wo.Technologies, []string{"go"}) || wo.Seniority != "unknown" || wo.WorkArrangement != "unknown" {
		t.Errorf("without model: %+v", wo)
	}
}

func TestExplanationQueueOnlyEnrichedStrongUnexplainedMatches(t *testing.T) {
	ef := newEnrichFixture(t)
	pool := requireDB(t)
	ctx := context.Background()
	research := NewResearchRepository(pool)
	profile, err := research.CreateProfile(ctx, ef.workspace.ID, ProfileParams{
		Name: uniq("expl"), TitleKeywords: []string{"engineer"}, Technologies: []string{"go", "react", "rust"}, Remote: domain.RemoteAny,
	})
	if err != nil {
		t.Fatal(err)
	}
	match := func(j domain.Job, score int) {
		reasons := `[{"code":"tech","params":{"matched":"2","total":"3"}},{"code":"remote"}]`
		if _, err := pool.Exec(ctx, `INSERT INTO research_matches (profile_id, job_id, score, reasons) VALUES ($1, $2, $3, $4::jsonb)`, profile.ID, j.ID, score, reasons); err != nil {
			t.Fatal(err)
		}
	}
	strong := ef.effectiveJob(t, "Strong enriched", "unknown", "unknown", []string{"go"}, nil)
	weak := ef.effectiveJob(t, "Weak enriched", "unknown", "unknown", []string{"go"}, nil)
	plain := ef.effectiveJob(t, "Strong but not enriched", "unknown", "unknown", []string{"go"}, nil)
	done := ef.effectiveJob(t, "Strong already explained", "unknown", "unknown", []string{"go"}, nil)
	ai := `{"seniority":"senior","work_arrangement":"remote","country_code":"DE","technologies":["go","react"],"summary":"x"}`
	for _, j := range []domain.Job{strong, weak, done} {
		ef.enrich(t, j, ai)
	}
	match(strong, 90)
	match(weak, 55)
	match(plain, 95)
	match(done, 80)
	var hash string
	pool.QueryRow(ctx, `SELECT description_hash FROM jobs WHERE id = $1`, done.ID).Scan(&hash)
	if err := ef.repo.SaveExplanation(ctx, ef.workspace.ID, profile.ID, done.ID, hash, "expl-model", 1, "Already there."); err != nil {
		t.Fatal(err)
	}

	tasks, err := ef.repo.ExplanationQueue(ctx, "expl-model", 1, 70, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var mine []domain.ExplanationTask
	for _, tk := range tasks {
		if tk.WorkspaceID == ef.workspace.ID {
			mine = append(mine, tk)
		}
	}
	if len(mine) != 1 || mine[0].JobID != strong.ID {
		t.Fatalf("queue = %+v, want only the strong enriched unexplained match", mine)
	}
	tk := mine[0]
	if tk.ProfileID != profile.ID || tk.Score != 90 || tk.Title != "Strong enriched" || tk.DescriptionHash == "" {
		t.Errorf("task = %+v", tk)
	}
	if !slices.Equal(tk.Technologies, []string{"go", "react"}) || !slices.Equal(tk.MatchedTechnologies, []string{"go", "react"}) {
		t.Errorf("technologies = %v matched = %v", tk.Technologies, tk.MatchedTechnologies)
	}
	if tk.Seniority != "senior" || tk.Arrangement != "remote" || tk.Country != "DE" || len(tk.Reasons) != 2 {
		t.Errorf("facts = %+v", tk)
	}
	if tk.Locale != "tr" && tk.Locale != "en" {
		t.Errorf("locale = %q", tk.Locale)
	}
}

func TestAIFactsMarksOnlyWhatTheModelAdded(t *testing.T) {
	ef := newEnrichFixture(t)
	tr := "TR"
	rules := ef.effectiveJob(t, "Rules know it", "senior", "remote", []string{"go"}, &tr)
	gaps := ef.effectiveJob(t, "Model fills", "unknown", "unknown", []string{"go"}, nil)
	none := ef.effectiveJob(t, "No model", "unknown", "unknown", []string{"go"}, nil)
	ai := `{"seniority":"junior","work_arrangement":"hybrid","country_code":"DE","technologies":["go","rust"],"summary":"A Go and Rust role.","remote_policy":"2 days a week",
		"salary_min":90000,"salary_max":110000,"salary_currency":"EUR","salary_period":"year"}`
	ef.enrich(t, rules, ai)
	ef.enrich(t, gaps, ai)

	got, err := ef.repo.AIFacts(context.Background(), []string{rules.ID, gaps.ID, none.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got[none.ID]; ok {
		t.Fatal("a job the model never read has no AI facts")
	}
	r := got[rules.ID]
	if len(r.Fields) != 0 || !slices.Equal(r.ExtraTechnologies, []string{"rust"}) || r.Summary != "A Go and Rust role." || r.Model != testModel {
		t.Errorf("rules job: %+v (the rules knew seniority, arrangement and country, so none is the model's)", r)
	}
	g := got[gaps.ID]
	if !slices.Equal(g.Fields, []string{"seniority", "work_arrangement", "country"}) {
		t.Errorf("gap job fields = %v", g.Fields)
	}
	if g.SalaryMin == nil || *g.SalaryMin != 90000 || *g.SalaryMax != 110000 || g.SalaryCurrency != "EUR" || g.SalaryPeriod != "year" || g.RemotePolicy != "2 days a week" {
		t.Errorf("gap job = %+v", g)
	}
	if got, _ := ef.repo.AIFacts(context.Background(), nil); len(got) != 0 {
		t.Errorf("no ids returned %v", got)
	}
}

func TestExplanationTaskForServesAnyScoreAndOnlyTheWorkspacesOwnMatch(t *testing.T) {
	ef := newEnrichFixture(t)
	other := newEnrichFixture(t)
	pool := requireDB(t)
	ctx := context.Background()
	profile, err := NewResearchRepository(pool).CreateProfile(ctx, ef.workspace.ID, ProfileParams{
		Name: uniq("low"), TitleKeywords: []string{"engineer"}, Technologies: []string{"go"}, Remote: domain.RemoteAny,
	})
	if err != nil {
		t.Fatal(err)
	}
	weak := ef.effectiveJob(t, "Weak enriched", "unknown", "unknown", []string{"go"}, nil)
	unread := ef.effectiveJob(t, "Weak unread", "unknown", "unknown", []string{"go"}, nil)
	ef.enrich(t, weak, `{"seniority":"senior","work_arrangement":"remote","country_code":"DE","technologies":["go"],"summary":"x"}`)
	for _, j := range []domain.Job{weak, unread} {
		if _, err := pool.Exec(ctx, `INSERT INTO research_matches (profile_id, job_id, score, reasons) VALUES ($1, $2, 12, '[{"code":"remote"}]'::jsonb)`, profile.ID, j.ID); err != nil {
			t.Fatal(err)
		}
	}
	task, err := ef.repo.ExplanationTaskFor(ctx, ef.workspace.ID, profile.ID, weak.ID)
	if err != nil || task.Score != 12 || task.Title != "Weak enriched" || len(task.MatchedTechnologies) != 1 {
		t.Fatalf("task = %+v err = %v", task, err)
	}
	if _, err := ef.repo.ExplanationTaskFor(ctx, ef.workspace.ID, profile.ID, unread.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a job the model has not read: err = %v", err)
	}
	if _, err := ef.repo.ExplanationTaskFor(ctx, other.workspace.ID, profile.ID, weak.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("another workspace must not reach the match: err = %v", err)
	}
	if ok, err := ef.repo.HasEnrichment(ctx, weak.ID); err != nil || !ok {
		t.Fatalf("HasEnrichment weak = %v %v", ok, err)
	}
	if ok, _ := ef.repo.HasEnrichment(ctx, unread.ID); ok {
		t.Fatal("unread job reported as read")
	}
}

package httptransport

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/enrich"
)

type fakeLLM struct {
	calls  int
	probes int
	status enrich.Status
}

func (f *fakeLLM) Status(_ context.Context, probe bool) (enrich.Status, error) {
	f.calls++
	if probe {
		f.probes++
	}
	return f.status, nil
}

func adminApp(t *testing.T, d Deps) (admin, user *fiber.App) {
	t.Helper()
	au := testUser
	au.IsAdmin = true
	adminDeps, userDeps := d, d
	adminDeps.Auth = fakeAuthAs(au)
	adminDeps.Workspaces = allowAll
	userDeps.Workspaces = allowAll
	return newTestApp(adminDeps), newTestApp(userDeps)
}

func TestLLMStatusIsAdminOnlyAndShowsNoSecrets(t *testing.T) {
	now := time.Date(2031, 1, 1, 12, 0, 0, 0, time.UTC)
	f := &fakeLLM{status: enrich.Status{
		Enabled: true, BaseHost: "host.docker.internal:11434", Model: "qwen3:4b", FallbackModel: "qwen3:1.7b", Reachable: true, ModelPresent: true,
		Local: true, Queue: 120, Enriched: 30, Failed: 2, AvgLatencyMs: 3400, LastError: "timeout", LastCheckedAt: &now,
	}}
	admin, user := adminApp(t, Deps{LLM: f})

	expectError(t, call(t, user, "GET", "/api/v1/admin/llm", nil), 403, "forbidden")
	expectError(t, call(t, user, "POST", "/api/v1/admin/llm/check", nil), 403, "forbidden")
	if f.calls != 0 {
		t.Fatalf("a non-admin reached the provider (%d calls)", f.calls)
	}

	r := call(t, admin, "GET", "/api/v1/admin/llm", nil)
	expectStatus(t, r, 200)
	d := r.data()
	for k, want := range map[string]any{"enabled": true, "base_host": "host.docker.internal:11434", "model": "qwen3:4b", "fallback_model": "qwen3:1.7b",
		"reachable": true, "model_present": true, "degraded": false, "local": true, "last_error": "timeout"} {
		if d[k] != want {
			t.Errorf("%s = %v, want %v", k, d[k], want)
		}
	}
	if d["queue"].(float64) != 120 || d["enriched"].(float64) != 30 || d["failed"].(float64) != 2 || d["avg_latency_ms"].(float64) != 3400 || d["last_checked_at"] == nil {
		t.Errorf("counters = %v", d)
	}
	if f.probes != 0 {
		t.Fatal("GET must not run the model")
	}
	for _, leak := range []string{"api_key", "apiKey", "/v1", "sekret"} {
		if strings.Contains(r.Raw, leak) {
			t.Errorf("response mentions %q: %s", leak, r.Raw)
		}
	}
}

func TestLLMCheckProbesAndIsRateLimited(t *testing.T) {
	f := &fakeLLM{status: enrich.Status{Enabled: true, Reachable: true, ModelPresent: true, ProbeOK: true, ProbeMs: 900}}
	admin, _ := adminApp(t, Deps{LLM: f})
	r := call(t, admin, "POST", "/api/v1/admin/llm/check", nil)
	expectStatus(t, r, 200)
	if f.probes != 1 || r.data()["probe_ok"] != true || r.data()["probe_ms"].(float64) != 900 {
		t.Fatalf("probes = %d body = %s", f.probes, r.Raw)
	}
	expectError(t, call(t, admin, "POST", "/api/v1/admin/llm/check", nil), 429, "rate_limited")
	if f.probes != 1 {
		t.Fatalf("the rate-limited call still probed (%d)", f.probes)
	}
}

func TestLLMStatusWhenNotConfigured(t *testing.T) {
	admin, _ := adminApp(t, Deps{})
	r := call(t, admin, "GET", "/api/v1/admin/llm", nil)
	expectStatus(t, r, 200)
	if r.data()["enabled"] != false {
		t.Fatalf("body = %s", r.Raw)
	}
	expectStatus(t, call(t, admin, "POST", "/api/v1/admin/llm/check", nil), 200)
}

type fakeAI struct {
	facts        map[string]domain.JobAI
	explanations map[string]string
	gotProfile   string
}

func (f *fakeAI) AIFacts(context.Context, []string) (map[string]domain.JobAI, error) {
	return f.facts, nil
}
func (f *fakeAI) Explanations(_ context.Context, _, profileID string, _ []string) (map[string]string, error) {
	f.gotProfile = profileID
	return f.explanations, nil
}

func TestJobsCarryAIFieldsAndExplanations(t *testing.T) {
	min, max := 90000, 110000
	jobs := &fakeWorkspaceJobs{result: []domain.WorkspaceJob{
		{Job: domain.Job{ID: jobID, Title: "With AI"}, Match: &domain.Match{Score: 88}},
		{Job: domain.Job{ID: "33333333-3333-4333-8333-333333333333", Title: "No AI"}, Match: &domain.Match{Score: 70}},
	}}
	ai := &fakeAI{
		facts: map[string]domain.JobAI{jobID: {JobID: jobID, Model: "qwen3:4b", Summary: "A Go role.", RemotePolicy: "EU only",
			SalaryMin: &min, SalaryMax: &max, SalaryCurrency: "EUR", SalaryPeriod: "year", Fields: []string{"seniority"}, ExtraTechnologies: []string{"rust"}}},
		explanations: map[string]string{jobID: "Fits your Go search."},
	}
	res := &fakeResearch{profile: domain.ResearchProfile{ID: profileID}}
	app := newTestApp(Deps{Jobs: jobs, Research: res, Enrichments: ai, Workspaces: allowAll})

	r := call(t, app, "GET", wsPath+"/jobs?profile="+profileID+"&sort=score", nil)
	expectStatus(t, r, 200)
	with := r.list()[0].(map[string]any)
	aiBody := with["ai"].(map[string]any)
	if aiBody["model"] != "qwen3:4b" || aiBody["summary"] != "A Go role." || aiBody["remote_policy"] != "EU only" ||
		aiBody["fields"].([]any)[0] != "seniority" || aiBody["extra_technologies"].([]any)[0] != "rust" {
		t.Fatalf("ai = %v", aiBody)
	}
	sal := aiBody["salary"].(map[string]any)
	if sal["min"].(float64) != 90000 || sal["max"].(float64) != 110000 || sal["currency"] != "EUR" || sal["period"] != "year" {
		t.Fatalf("salary = %v", sal)
	}
	if with["match"].(map[string]any)["explanation"] != "Fits your Go search." || ai.gotProfile != profileID {
		t.Fatalf("explanation = %v (profile %q)", with["match"], ai.gotProfile)
	}
	without := r.list()[1].(map[string]any)
	if without["ai"] != nil || without["match"].(map[string]any)["explanation"] != nil {
		t.Fatalf("a job the model never read must have no ai fields: %v", without)
	}
}

func TestJobsWithoutEnrichmentsBehaveAsBefore(t *testing.T) {
	jobs := &fakeWorkspaceJobs{result: []domain.WorkspaceJob{{Job: domain.Job{ID: jobID, Title: "Plain"}}}}
	app := newTestApp(Deps{Jobs: jobs, Workspaces: allowAll})
	r := call(t, app, "GET", wsPath+"/jobs", nil)
	expectStatus(t, r, 200)
	if r.list()[0].(map[string]any)["ai"] != nil {
		t.Fatalf("body = %s", r.Raw)
	}
}

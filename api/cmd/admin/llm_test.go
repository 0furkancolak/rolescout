package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/llm/llmtest"
)

const evalReply = `{"technologies":["go"],"seniority":"senior","work_arrangement":"remote","remote_policy":"","employment_type":"unknown","salary_min":null,"salary_max":null,"salary_currency":"","salary_period":"unknown","location":"","country_code":"","languages":[],"years_experience":null,"summary":""}`

func llmEnv(t *testing.T, srv *llmtest.Server) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://unused")
	t.Setenv("LLM_ENABLED", "true")
	t.Setenv("LLM_BASE_URL", srv.BaseURL())
	t.Setenv("LLM_MODEL", "m")
	t.Setenv("LLM_TIMEOUT", "5s")
}

func TestLLMCheckReportsReachabilityAndLatency(t *testing.T) {
	srv := llmtest.New(t, "m")
	srv.Script(llmtest.Scripted{Content: "ok"})
	llmEnv(t, srv)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"llm", "check"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"reachable: yes", "model present: yes", "answered in"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "sekret") {
		t.Error("the key must never be printed")
	}
}

func TestLLMCheckFailsWhenTheServerIsDown(t *testing.T) {
	srv := llmtest.New(t, "m")
	llmEnv(t, srv)
	t.Setenv("LLM_BASE_URL", "http://127.0.0.1:1/v1")
	var out bytes.Buffer
	if err := run(context.Background(), []string{"llm", "check"}, &out); err == nil {
		t.Fatalf("a dead server must fail the check:\n%s", out.String())
	}
}

func TestLLMEvalPrintsAReport(t *testing.T) {
	srv := llmtest.New(t, "m")
	srv.Script(llmtest.Scripted{Content: evalReply})
	llmEnv(t, srv)
	dir := t.TempDir()
	cases := `[{"id":"a","lang":"en","title":"Senior Go Developer","company":"A","description":"Senior Go Developer wanted for a remote role. We use Go every day.",
	  "expect":{"seniority":"senior","work_arrangement":"remote","technologies":["go"]}},
	 {"id":"b","lang":"tr","title":"Kıdemli Go Geliştirici","company":"B","description":"Kıdemli Go geliştirici arıyoruz, uzaktan çalışma. Go kullanıyoruz.",
	  "expect":{"seniority":"senior","work_arrangement":"remote","technologies":["go"]}}]`
	if err := os.WriteFile(filepath.Join(dir, "set.json"), []byte(cases), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := run(ctx, []string{"llm", "eval", "-set", dir, "-limit", "2"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"model: m", "cases: 2", "seniority accuracy", "technologies precision", "median"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, out.String())
		}
	}
}

func TestLLMEvalHonoursLimitAndModelFlag(t *testing.T) {
	srv := llmtest.New(t, "m", "other")
	srv.Script(llmtest.Scripted{Content: evalReply})
	llmEnv(t, srv)
	dir := t.TempDir()
	one := `[{"id":"a","lang":"en","title":"T","company":"A","description":"A senior role using Go that is remote and has many words so it is long enough.","expect":{"seniority":"senior","work_arrangement":"remote","technologies":["go"]}},
	{"id":"b","lang":"en","title":"T","company":"A","description":"Another one using Go that is remote and has many words so it is long enough too.","expect":{"seniority":"senior","work_arrangement":"remote","technologies":["go"]}}]`
	os.WriteFile(filepath.Join(dir, "x.json"), []byte(one), 0o600)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"llm", "eval", "-set", dir, "-limit", "1", "-model", "other"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "cases: 1") || !strings.Contains(out.String(), "model: other") {
		t.Fatalf("output:\n%s", out.String())
	}
	if srv.Chats() != 1 || !strings.Contains(string(srv.Requests()[0].Body), `"model":"other"`) {
		t.Fatalf("chats = %d", srv.Chats())
	}
}

func TestLLMUsageErrors(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"llm"}, &out); err == nil {
		t.Fatal("llm without a subcommand must print usage")
	}
	if err := run(context.Background(), []string{"llm", "nope"}, &out); err == nil {
		t.Fatal("unknown subcommand must fail")
	}
}

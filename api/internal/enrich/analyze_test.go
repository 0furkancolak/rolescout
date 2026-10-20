package enrich

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/config"
	"example.com/rolescout/api/internal/domain"
	"example.com/rolescout/api/internal/llm"
	"example.com/rolescout/api/internal/llm/llmtest"
)

type fakeAnalysisRepo struct {
	fakeRepo
	read     bool
	task     domain.ExplanationTask
	taskErr  error
	explains []string
}

func (f *fakeAnalysisRepo) EnrichmentTaskFor(_ context.Context, id string) (domain.EnrichmentTask, error) {
	return tasks(1)[0], nil
}
func (f *fakeAnalysisRepo) HasEnrichment(context.Context, string) (bool, error) { return f.read, nil }
func (f *fakeAnalysisRepo) ExplanationTaskFor(context.Context, string, string, string) (domain.ExplanationTask, error) {
	return f.task, f.taskErr
}
func (f *fakeAnalysisRepo) SaveExplanation(_ context.Context, _, _, _, _, model string, _ int, text string) error {
	f.explains = append(f.explains, model+"|"+text)
	return nil
}

func analyzer(t *testing.T, enabled bool, repo *fakeAnalysisRepo, replies ...llmtest.Scripted) (*Analyzer, *llmtest.Server) {
	t.Helper()
	srv := llmtest.New(t)
	srv.Script(replies...)
	cl := llm.NewClient(llm.Options{BaseURL: srv.BaseURL(), Model: "m1", APIKey: "k", Timeout: 5 * time.Second})
	cfg := config.LLM{Enabled: enabled, Model: "m1", MaxInputChars: 6000, MaxOutputTokens: 400}
	return &Analyzer{Repo: repo, LLM: cl, Resolve: func(context.Context) config.LLM { return cfg }}, srv
}

func lowMatch() domain.ExplanationTask {
	return domain.ExplanationTask{
		WorkspaceID: "w", ProfileID: "p", JobID: "job-0", DescriptionHash: "hash-0", Title: "Engineer", CompanyName: "Acme",
		Score: 35, Technologies: []string{"go"}, MatchedTechnologies: []string{"go"}, Locale: "en",
		Reasons: []domain.MatchReason{{Code: "technology", Params: map[string]string{"value": "go"}}},
	}
}

func TestAnalyzeReadsThePostingThenExplainsALowScore(t *testing.T) {
	repo := &fakeAnalysisRepo{task: lowMatch()}
	a, srv := analyzer(t, true, repo,
		llmtest.Scripted{Content: goodReply},
		llmtest.Scripted{Content: `{"explanation":"The job uses go, which is in your search."}`})
	text, err := a.Analyze(context.Background(), "w", "p", "job-0", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "go") || srv.Chats() != 2 {
		t.Fatalf("text %q chats %d", text, srv.Chats())
	}
	if len(repo.saved) != 1 || !repo.saved[0].OK || repo.saved[0].Model != "m1" {
		t.Fatalf("enrichment saved = %+v", repo.saved)
	}
	if len(repo.explains) != 1 || !strings.HasPrefix(repo.explains[0], "m1|") {
		t.Fatalf("explanations = %v", repo.explains)
	}
	// The API key reaches the service as a bearer token.
	if got := srv.Requests()[0].Header.Get("Authorization"); got != "Bearer k" {
		t.Fatalf("authorization = %q", got)
	}
}

func TestAnalyzeSkipsReadingWhenAlreadyRead(t *testing.T) {
	repo := &fakeAnalysisRepo{task: lowMatch(), read: true}
	a, srv := analyzer(t, true, repo, llmtest.Scripted{Content: `{"explanation":"The job uses go, which is in your search."}`})
	if _, err := a.Analyze(context.Background(), "w", "p", "job-0", ""); err != nil {
		t.Fatal(err)
	}
	if srv.Chats() != 1 {
		t.Fatalf("chats = %d, want only the explanation", srv.Chats())
	}
}

func TestAnalyzeDoesNothingWhileAiIsOff(t *testing.T) {
	repo := &fakeAnalysisRepo{task: lowMatch()}
	a, srv := analyzer(t, false, repo)
	if _, err := a.Analyze(context.Background(), "w", "p", "job-0", ""); !errors.Is(err, ErrDisabled) {
		t.Fatalf("err = %v", err)
	}
	if srv.Chats() != 0 {
		t.Fatal("no request may be sent")
	}
}

func TestAnalyzeRejectsUngroundedText(t *testing.T) {
	repo := &fakeAnalysisRepo{task: lowMatch(), read: true}
	a, _ := analyzer(t, true, repo, llmtest.Scripted{Content: `{"explanation":"A great Rust role paying 200000."}`})
	if _, err := a.Analyze(context.Background(), "w", "p", "job-0", ""); !errors.Is(err, llm.ErrInvalidOutput) {
		t.Fatalf("err = %v", err)
	}
	if len(repo.explains) != 0 {
		t.Fatal("an ungrounded answer must not be stored")
	}
}

func TestAnalyzeReportsAnUnreachableServiceAndBadKey(t *testing.T) {
	repo := &fakeAnalysisRepo{task: lowMatch(), read: true}
	a, _ := analyzer(t, true, repo, llmtest.Scripted{Status: 401})
	if _, err := a.Analyze(context.Background(), "w", "p", "job-0", ""); !errors.Is(err, llm.ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestAnalyzeRefusesAParallelRunOfTheSameJob(t *testing.T) {
	repo := &fakeAnalysisRepo{task: lowMatch(), read: true}
	a, _ := analyzer(t, true, repo, llmtest.Scripted{Content: `{"explanation":"The job uses go, which is in your search."}`, Delay: 300 * time.Millisecond})
	done := make(chan error, 1)
	go func() { _, err := a.Analyze(context.Background(), "w", "p", "job-0", ""); done <- err }()
	time.Sleep(100 * time.Millisecond)
	if _, err := a.Analyze(context.Background(), "w", "p", "job-0", ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundDefaultsFollowWhereTheModelRuns(t *testing.T) {
	no, yes := false, true
	cases := []struct {
		name string
		cfg  config.LLM
		want bool
	}{
		{"ollama on the host", config.LLM{BaseURL: "http://host.docker.internal:11434/v1"}, true},
		{"private network", config.LLM{BaseURL: "http://192.168.1.5:8080/v1"}, true},
		{"hosted service", config.LLM{BaseURL: "https://api.openai.com/v1"}, false},
		{"hosted but chosen", config.LLM{BaseURL: "https://api.openai.com/v1", Background: &yes}, true},
		{"local but off", config.LLM{BaseURL: "http://127.0.0.1:11434/v1", Background: &no}, false},
	}
	for _, c := range cases {
		if got := BackgroundOn(c.cfg); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestTickSkipsTheCatalogForAHostedServiceUnlessChosen(t *testing.T) {
	h := newHarness(t, func(e *Enricher) {
		e.Resolve = func(context.Context) config.LLM {
			return config.LLM{Enabled: true, BaseURL: "https://api.openai.com/v1", Model: "m"}
		}
	})
	h.repo.queue = tasks(3)
	res, err := h.e.Tick(context.Background())
	if err != nil || !res.Skipped || len(h.repo.queue) != 3 || h.srv.Chats() != 0 {
		t.Fatalf("res %+v err %v queue %d chats %d", res, err, len(h.repo.queue), h.srv.Chats())
	}
}

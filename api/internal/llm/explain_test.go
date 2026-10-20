package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/llm/llmtest"
)

func facts() JobFacts {
	return JobFacts{
		Title: "Senior Full Stack Engineer", Company: "Acme", Score: 88,
		Reasons:      []string{"the title says full stack", "3 of your 5 technologies", "remote"},
		Technologies: []string{"react", "typescript", "node.js", "go"}, MatchedTechnologies: []string{"react", "typescript", "node.js"},
		Seniority: "senior", Arrangement: "remote", Country: "DE", Language: "en",
	}
}

func explainer(t *testing.T, reply string) (*Client, *llmtest.Server) {
	t.Helper()
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: reply})
	return NewClient(Options{BaseURL: srv.BaseURL(), Model: "m", Timeout: 2 * time.Second}), srv
}

func TestExplainReturnsTwoSentencesFromTheFacts(t *testing.T) {
	c, srv := explainer(t, `{"explanation":"It is a remote senior full stack role that uses React, TypeScript and Node.js. Your title keyword matches directly."}`)
	got, err := Explain(context.Background(), c, facts())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "It is a remote senior full stack role") {
		t.Fatalf("explanation = %q", got)
	}
	body := string(srv.Requests()[0].Body)
	for _, want := range []string{`"json_schema"`, "Senior Full Stack Engineer", "react", "English"} {
		if !strings.Contains(body, want) {
			t.Errorf("request lacks %q: %s", want, body)
		}
	}
}

func TestExplainSpeaksTheUsersLanguage(t *testing.T) {
	c, srv := explainer(t, `{"explanation":"Uzaktan çalışılan kıdemli bir full stack rolü."}`)
	f := facts()
	f.Language = "tr"
	if _, err := Explain(context.Background(), c, f); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(srv.Requests()[0].Body), "Turkish") {
		t.Fatalf("prompt does not ask for Turkish: %s", srv.Requests()[0].Body)
	}
}

func TestExplainRejectsInventedTechnologies(t *testing.T) {
	c, _ := explainer(t, `{"explanation":"It uses React and Rust, which fits your skills."}`)
	if _, err := Explain(context.Background(), c, facts()); !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("err = %v, want ErrInvalidOutput (Rust is not in the facts)", err)
	}
}

func TestExplainRejectsInventedNumbers(t *testing.T) {
	for _, reply := range []string{
		`{"explanation":"It pays 150,000 a year."}`,
		`{"explanation":"The team has 12 engineers."}`,
	} {
		c, _ := explainer(t, reply)
		if _, err := Explain(context.Background(), c, facts()); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("%s: err = %v, want ErrInvalidOutput", reply, err)
		}
	}
	// Numbers that are in the facts are fine: the score and "3 of your 5".
	c, _ := explainer(t, `{"explanation":"It scores 88 and matches 3 of your 5 technologies."}`)
	if _, err := Explain(context.Background(), c, facts()); err != nil {
		t.Fatalf("grounded numbers rejected: %v", err)
	}
}

func TestExplainRejectsTooLongOrEmpty(t *testing.T) {
	for name, reply := range map[string]string{
		"four sentences": `{"explanation":"One. Two. Three. Four."}`,
		"empty":          `{"explanation":"  "}`,
		"not json":       `Sure!`,
		"too long":       `{"explanation":"` + strings.Repeat("word ", 90) + `."}`,
	} {
		c, _ := explainer(t, reply)
		if _, err := Explain(context.Background(), c, facts()); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("%s: err = %v, want ErrInvalidOutput", name, err)
		}
	}
}

func TestExplainPassesTransportErrorsThrough(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Status: 500})
	c := NewClient(Options{BaseURL: srv.BaseURL(), Model: "m", Timeout: time.Second})
	if _, err := Explain(context.Background(), c, facts()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestExplainTellsTheModelWhatTheCVLacks(t *testing.T) {
	c, srv := explainer(t, `{"explanation":"Fits your Go work, but it also asks for PHP, which you lack."}`)
	f := facts()
	f.Technologies = []string{"go", "php"}
	f.MatchedTechnologies = []string{"go"}
	f.MissingTechnologies = []string{"php"}
	f.CVSkills = []string{"go", "docker"}
	if _, err := Explain(context.Background(), c, f); err != nil {
		t.Fatal(err)
	}
	reqs := srv.Requests()
	body := string(reqs[len(reqs)-1].Body)
	if !strings.Contains(body, "the person lacks: php") || !strings.Contains(body, "CV skills: go, docker") {
		t.Errorf("prompt does not carry the gap: %s", body)
	}
}

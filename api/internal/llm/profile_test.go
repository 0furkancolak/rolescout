package llm

import (
	"context"
	"errors"
	"testing"

	"example.com/rolescout/api/internal/llm/llmtest"
)

func TestSuggestProfileCleansTheDraft(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"name":"  Go backend, Berlin ","title_keywords":["backend engineer","backend engineer",""],"keywords":["api"],"technologies":["Go","PostgreSQL","go"],"countries":["de","Germany","tr"],"locations":["Berlin"],"excluded_keywords":["php"],"remote":"sometimes","seniority":["Senior","wizard","senior"]}`})
	c := newTestClient(t, srv)
	s, err := SuggestProfile(context.Background(), c, "I want a senior Go backend job in Berlin, no PHP")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Go backend, Berlin" || len(s.TitleKeywords) != 1 || len(s.Technologies) != 2 || s.Technologies[0] != "go" {
		t.Errorf("draft = %+v", s)
	}
	if len(s.Countries) != 2 || s.Countries[0] != "DE" || s.Countries[1] != "TR" {
		t.Errorf("countries = %v", s.Countries)
	}
	if s.Remote != "any" || len(s.Seniority) != 1 || s.Seniority[0] != "senior" {
		t.Errorf("remote/seniority = %q %v", s.Remote, s.Seniority)
	}
}

func TestSuggestProfileRejectsAnEmptyDraft(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"name":"","title_keywords":[],"keywords":[],"technologies":[],"countries":[],"locations":[],"excluded_keywords":[],"remote":"any","seniority":[]}`})
	_, err := SuggestProfile(context.Background(), newTestClient(t, srv), "something vague here")
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("err = %v", err)
	}
}

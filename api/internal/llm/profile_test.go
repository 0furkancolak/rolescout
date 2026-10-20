package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"example.com/rolescout/api/internal/llm/llmtest"
)

func TestSuggestProfileCleansTheDraft(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"name":"  Go backend, Berlin ","title_keywords":["backend engineer","backend engineer",""],"keywords":["api"],"technologies":["Go","PostgreSQL","go"],"countries":["de","Germany","tr"],"locations":["Berlin"],"excluded_keywords":["php"],"remote":"sometimes","seniority":["Senior","wizard","senior"]}`})
	c := newTestClient(t, srv)
	s, err := SuggestProfile(context.Background(), c, "I want a senior Go backend job in Berlin, no PHP", Background{})
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
	_, err := SuggestProfile(context.Background(), newTestClient(t, srv), "something vague here", Background{})
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("err = %v", err)
	}
}

func TestSuggestProfileUsesTheBackgroundAndKeepsExcludedOutOfTechnologies(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"name":"Backend","title_keywords":["backend developer","yazılım geliştirici"],"keywords":["microservices"],"technologies":["go","docker"],"countries":["TR"],"locations":["İstanbul"],"excluded_keywords":["sales","docker"],"remote":"any","seniority":["mid","senior"]}`})
	s, err := SuggestProfile(context.Background(), newTestClient(t, srv), "", Background{
		About: "Backend işleri arıyorum", CVSkills: []string{"go", "docker"}, CVSeniority: "senior", CVYears: 6,
		ExistingProfiles: []string{"Full Stack Developer (full stack)"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ExcludedKeywords) != 1 || s.ExcludedKeywords[0] != "sales" {
		t.Errorf("a technology the profile asks for must not be excluded: %v", s.ExcludedKeywords)
	}
	reqs := srv.Requests()
	body := string(reqs[len(reqs)-1].Body)
	for _, want := range []string{"Backend işleri arıyorum", "CV skills: go, docker", "6 years", "Full Stack Developer"} {
		if !strings.Contains(body, want) {
			t.Errorf("the model was not told %q", want)
		}
	}
}

func TestParseCVDropsSkillsTheCVDoesNotContain(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"titles":["Backend Developer"],"skills":["Go","PostgreSQL","kubernetes"],"seniority":"Senior","years_experience":6,"summary":"Backend geliştirici."}`})
	p, err := ParseCV(context.Background(), newTestClient(t, srv), "Backend Developer. Go ve PostgreSQL ile çalıştım.")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Skills) != 2 || p.Seniority != "senior" || p.Years != 6 {
		t.Fatalf("cv = %+v", p)
	}
}

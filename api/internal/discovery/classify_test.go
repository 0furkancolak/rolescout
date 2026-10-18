package discovery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/ingest/ingesttest"
	"example.com/rolescout/api/internal/llm"
	"example.com/rolescout/api/internal/llm/llmtest"
)

type pickFn func(ctx context.Context, title string, c []LinkCandidate) (int, bool, error)

func (f pickFn) PickLink(ctx context.Context, title string, c []LinkCandidate) (int, bool, error) {
	return f(ctx, title, c)
}

// A page the rules cannot place: no embedded board, no schema.org data and no
// link that looks like a careers page, but a link the model can recognise.
func oddSite(extra map[string]string) map[string]string {
	pages := map[string]string{
		"odd.example/":            `<html><title>Odd Co</title><a href="/about">About us</a><a href="/we-build">What we build</a><a href="/people/team">Come build with us</a><a href="/blog">Blog</a></html>`,
		"odd.example/people/team": `<html><iframe src="https://jobs.lever.co/oddco"></iframe></html>`,
	}
	for k, v := range extra {
		pages[k] = v
	}
	return pages
}

func TestClassifierPicksTheCareersLinkAndTheBoardIsDetected(t *testing.T) {
	d := sites(t, oddSite(nil))
	var seen []LinkCandidate
	d.Classifier = pickFn(func(_ context.Context, title string, c []LinkCandidate) (int, bool, error) {
		seen = c
		for i, cand := range c {
			if strings.Contains(cand.URL, "/people/team") {
				return i, true, nil
			}
		}
		return 0, false, nil
	})
	got, err := d.Detect(ingesttest.Ctx(t), "http://odd.example/")
	if err != nil || got.Board.ATS != "lever" || got.Board.ID != "oddco" {
		t.Fatalf("detection = %+v, err = %v", got, err)
	}
	if len(seen) == 0 || len(seen) > 12 || seen[0].URL == "" || seen[0].Text == "" {
		t.Fatalf("candidates = %+v", seen)
	}
}

func TestWithoutAClassifierDetectionIsUnchanged(t *testing.T) {
	d := sites(t, oddSite(nil))
	got, err := d.Detect(ingesttest.Ctx(t), "http://odd.example/")
	if err != nil || got.Board.ATS != "unknown" {
		t.Fatalf("detection = %+v, err = %v", got, err)
	}
}

func TestClassifierPickThatIsNotABoardChangesNothing(t *testing.T) {
	d := sites(t, oddSite(map[string]string{"odd.example/people/team": `<html><p>Just a team photo</p></html>`}))
	d.Classifier = pickFn(func(_ context.Context, _ string, c []LinkCandidate) (int, bool, error) { return 2, true, nil })
	got, err := d.Detect(ingesttest.Ctx(t), "http://odd.example/")
	if err != nil || got.Board.ATS != "unknown" {
		t.Fatalf("a wrong pick must leave the detection unknown: %+v, err = %v", got, err)
	}
}

func TestClassifierErrorsAndBadIndexesAreIgnored(t *testing.T) {
	for name, pick := range map[string]pickFn{
		"error": func(context.Context, string, []LinkCandidate) (int, bool, error) {
			return 0, false, errors.New("model down")
		},
		"negative":  func(context.Context, string, []LinkCandidate) (int, bool, error) { return -1, true, nil },
		"too large": func(context.Context, string, []LinkCandidate) (int, bool, error) { return 99, true, nil },
		"no choice": func(context.Context, string, []LinkCandidate) (int, bool, error) { return 0, false, nil },
	} {
		d := sites(t, oddSite(nil))
		d.Classifier = pick
		got, err := d.Detect(ingesttest.Ctx(t), "http://odd.example/")
		if err != nil || got.Board.ATS != "unknown" {
			t.Errorf("%s: detection = %+v, err = %v", name, got, err)
		}
	}
}

func TestClassifierIsNotAskedWhenTheRulesFindABoard(t *testing.T) {
	d := sites(t, map[string]string{"acme.example/careers": `<html><iframe src="https://jobs.lever.co/acme"></iframe></html>`})
	d.Classifier = pickFn(func(context.Context, string, []LinkCandidate) (int, bool, error) {
		t.Error("classifier was asked although the rules found the board")
		return 0, false, nil
	})
	if got, _ := d.Detect(ingesttest.Ctx(t), "http://acme.example/careers"); got.Board.ATS != "lever" {
		t.Fatalf("detection = %+v", got)
	}
}

func TestLLMClassifierChoosesAnIndex(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: `{"index":2}`})
	c := &LLMClassifier{LLM: llm.NewClient(llm.Options{BaseURL: srv.BaseURL(), Model: "m", Timeout: 2 * time.Second})}
	cands := []LinkCandidate{{URL: "http://x/a", Text: "About"}, {URL: "http://x/b", Text: "Blog"}, {URL: "http://x/join", Text: "Come build with us"}}
	i, ok, err := c.PickLink(context.Background(), "Odd Co", cands)
	if err != nil || !ok || i != 2 {
		t.Fatalf("pick = %d %v %v", i, ok, err)
	}
	body := string(srv.Requests()[0].Body)
	if !strings.Contains(body, "Come build with us") || !strings.Contains(body, "/join") || strings.Contains(body, "http://x/join") {
		t.Fatalf("prompt must show link text and path, not invent or leak full URLs: %s", body)
	}
}

func TestLLMClassifierRejectsOutOfRangeAndMalformedAnswers(t *testing.T) {
	cands := []LinkCandidate{{URL: "http://x/a", Text: "A"}, {URL: "http://x/b", Text: "B"}}
	for name, reply := range map[string]string{
		"out of range": `{"index":7}`, "null": `{"index":null}`, "negative": `{"index":-1}`, "not json": `maybe the second one`,
	} {
		srv := llmtest.New(t)
		srv.Script(llmtest.Scripted{Content: reply})
		c := &LLMClassifier{LLM: llm.NewClient(llm.Options{BaseURL: srv.BaseURL(), Model: "m", Timeout: 2 * time.Second})}
		if _, ok, err := c.PickLink(context.Background(), "t", cands); ok || err != nil {
			t.Errorf("%s: ok=%v err=%v, want no choice and no error", name, ok, err)
		}
	}
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Status: 500})
	c := &LLMClassifier{LLM: llm.NewClient(llm.Options{BaseURL: srv.BaseURL(), Model: "m", Timeout: time.Second})}
	if _, _, err := c.PickLink(context.Background(), "t", cands); !errors.Is(err, llm.ErrUnavailable) {
		t.Fatalf("transport errors must be returned, got %v", err)
	}
}

package httptransport

import (
	"context"
	"testing"

	"example.com/rolescout/api/internal/enrich"
	"example.com/rolescout/api/internal/llm"
)

type stubSuggester struct {
	err  error
	last string
}

func (s *stubSuggester) SuggestProfile(_ context.Context, _, d string) (llm.ProfileSuggestion, error) {
	s.last = d
	if s.err != nil {
		return llm.ProfileSuggestion{}, s.err
	}
	return llm.ProfileSuggestion{Name: "Go backend", Technologies: []string{"go"}, Remote: "remote_only"}, nil
}

func TestSuggestProfileReturnsADraftWithoutSavingIt(t *testing.T) {
	st := &stubSuggester{}
	app := newTestApp(Deps{Auth: fakeAuth{}, Suggester: st, Workspaces: allowAll})
	path := wsPath + "/research-profiles/suggest"
	r := call(t, app, "POST", path, map[string]string{"description": "remote Go backend roles"})
	expectStatus(t, r, 200)
	d := r.data()
	if d["name"] != "Go backend" || d["remote"] != "remote_only" || d["schedule_hours"] != float64(12) {
		t.Fatalf("draft = %v", d)
	}
	if l, ok := d["keywords"].([]any); !ok || len(l) != 0 {
		t.Errorf("empty lists must be [], got %v", d["keywords"])
	}
	if st.last != "remote Go backend roles" {
		t.Errorf("model got %q", st.last)
	}
	expectStatus(t, call(t, app, "POST", path, map[string]string{"description": "short"}), 422)
}

func TestSuggestProfileReportsAiStates(t *testing.T) {
	path := wsPath + "/research-profiles/suggest"
	body := map[string]string{"description": "remote Go backend roles"}
	for err, want := range map[error]int{enrich.ErrDisabled: 412, llm.ErrUnavailable: 503, llm.ErrInvalidOutput: 502} {
		app := newTestApp(Deps{Auth: fakeAuth{}, Suggester: &stubSuggester{err: err}, Workspaces: allowAll})
		expectStatus(t, call(t, app, "POST", path, body), want)
	}
	app := newTestApp(Deps{Auth: fakeAuth{}, Workspaces: allowAll})
	expectStatus(t, call(t, app, "POST", path, body), 412)
}

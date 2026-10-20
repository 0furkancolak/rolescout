package hnhiring

import (
	"net/http"
	"strings"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	if a.Validate(map[string]any{"threads": "9"}) == nil {
		t.Error("threads 9 accepted")
	}
	if err := a.Validate(map[string]any{"threads": "2"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchReadsTopLevelPostsOfTheNewestThread(t *testing.T) {
	var comment string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		tags := r.URL.Query().Get("tags")
		switch {
		case strings.HasPrefix(tags, "story"):
			w.Write([]byte(`{"hits":[{"objectID":"3","title":"Ask HN: Who wants to be hired? (October 2026)"},{"objectID":"2","title":"Ask HN: Who is hiring? (October 2026)"},{"objectID":"1","title":"Ask HN: Who is hiring? (September 2026)"}]}`))
		case tags == "comment,story_2":
			comment = tags
			w.Write([]byte(`{"nbPages":1,"hits":[
{"objectID":"20","parent_id":2,"story_id":2,"created_at_i":1792400000,"comment_text":"Acme &#x2F; Labs | Senior Go Engineer | Berlin, Germany | REMOTE (EU) | Full-time<p>We build things with Go and PostgreSQL. <a href=\"https://acme.example/jobs\">Apply</a>"},
{"objectID":"21","parent_id":20,"story_id":2,"created_at_i":1792400100,"comment_text":"how do I apply?"},
{"objectID":"22","parent_id":2,"story_id":2,"created_at_i":1792400200,"comment_text":"Not a header line at all<p>just text"}]}`))
		default:
			t.Errorf("unexpected tags %q", tags)
		}
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
	if err == nil || len(got) != 1 || comment == "" {
		t.Fatalf("got %d listings, err %v", len(got), err)
	}
	l := got[0]
	if l.ExternalID != "20" || l.Company != "Acme / Labs" || l.Title != "Senior Go Engineer" ||
		l.Location != "Berlin, Germany" || l.Arrangement != "remote" ||
		l.URL != "https://news.ycombinator.com/item?id=20" || l.PostedAt == nil || !strings.Contains(l.Description, "PostgreSQL") {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
		return err
	})
}

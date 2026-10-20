package gem

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	for _, bad := range []map[string]any{{}, {"board": "a/b"}, {"board": "gem", "x": 1}} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	if err := a.Validate(map[string]any{"board": "gem"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/jobs.json")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"board": "gem", "company": "Gem"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/job_board/v0/gem/job_posts/" || len(got) != 2 {
		t.Fatalf("path %s, %d listings", path, len(got))
	}
	l := got[0]
	if l.ExternalID != "4965519002" || l.Title != "Software Engineer" || l.Company != "Gem" ||
		l.URL != "https://jobs.gem.com/gem/4965519002" || l.Location != "San Francisco, United States" ||
		l.Arrangement != "hybrid" || l.EmploymentType != "full time" ||
		l.PostedAt == nil || l.PostedAt.Format("2006-01-02") != "2020-11-17" || l.Description == "" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"board": "acme"})
		return err
	})
}

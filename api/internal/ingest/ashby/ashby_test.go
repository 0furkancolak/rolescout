package ashby

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	for _, bad := range []map[string]any{{}, {"board": ""}, {"board": "a b"}, {"board": "acme", "x": 1}} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	if err := a.Validate(map[string]any{"board": "Acme.Inc", "company": "Acme"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMapsFixture(t *testing.T) {
	var path string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.RequestURI()
		ingesttest.File(t, "testdata/board.json")(w, r)
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"board": "ashby", "company": "Ashby"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/posting-api/job-board/ashby?includeCompensation=false" {
		t.Errorf("path = %s", path)
	}
	if len(got) == 0 {
		t.Fatal("no listings")
	}
	l := got[0]
	if l.ExternalID != "7458d4e9-da2e-47bd-98cb-adfda43d42b2" || l.Title != "Engineering Manager - EU" ||
		l.URL != "https://jobs.ashbyhq.com/ashby/7458d4e9-da2e-47bd-98cb-adfda43d42b2" || l.Company != "Ashby" ||
		l.Location != "Remote - European Union" || l.Arrangement != "remote" || l.EmploymentType != "Full-time" ||
		l.PostedAt == nil || l.Description == "" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"board": "acme"})
		return err
	})
}

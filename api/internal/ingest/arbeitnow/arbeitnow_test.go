package arbeitnow

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	if a.Validate(map[string]any{"max_pages": "x"}) == nil {
		t.Error("bad max_pages accepted")
	}
	if err := a.Validate(map[string]any{"max_pages": "5"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchFollowsPagesUpToTheLimit(t *testing.T) {
	var pages []string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.Query().Get("page"))
		w.Write([]byte(`{"data":[{"slug":"job-` + r.URL.Query().Get("page") + `","company_name":"likeyaa GmbH","title":"Projektmanager","description":"<p>Hallo</p>","remote":true,"url":"https://www.arbeitnow.com/jobs/x","job_types":["Vollzeit"],"location":"Dortmund","created_at":1792522843}],"links":{"next":"https://www.arbeitnow.com/api/job-board-api?page=9"}}`))
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"max_pages": "2"})
	if err == nil || len(got) != 2 || len(pages) != 2 || pages[1] != "2" {
		t.Fatalf("got %d listings from pages %v, err %v", len(got), pages, err)
	}
	l := got[0]
	if l.ExternalID != "job-1" || l.Company != "likeyaa GmbH" || l.Location != "Dortmund" ||
		l.Arrangement != "remote" || l.EmploymentType != "Vollzeit" || l.PostedAt == nil {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchStopsWithoutNextLink(t *testing.T) {
	n := 0
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Write([]byte(`{"data":[{"slug":"a","title":"T","company_name":"C","url":"https://x.example/a"}],"links":{"next":null}}`))
	})
	got, _ := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
	if n != 1 || len(got) != 1 {
		t.Errorf("requests = %d, listings = %d", n, len(got))
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
		return err
	})
}

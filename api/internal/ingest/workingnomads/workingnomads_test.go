package workingnomads

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

const fixture = `[{"url":"https://www.workingnomads.com/job/go/1958438/","title":"Senior DevOps Engineer","description":"<p>Terraform</p>","company_name":"Lemon.io","category_name":"Development","tags":"devops,python,golang","location":"Europe, LATAM","pub_date":"2026-10-20T13:39:57-04:00"},{"url":"","title":"broken"}]`

func TestFetchMapsFixture(t *testing.T) {
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/exposed_jobs/" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(fixture))
	})
	a := &Adapter{Client: c, BaseURL: base}
	if err := a.Validate(map[string]any{}); err != nil {
		t.Fatal(err)
	}
	got, err := a.Fetch(ingesttest.Ctx(t), map[string]any{})
	if err == nil || len(got) != 1 {
		t.Fatalf("got %d listings, err %v", len(got), err)
	}
	l := got[0]
	if l.Title != "Senior DevOps Engineer" || l.Company != "Lemon.io" || l.Location != "Europe, LATAM" ||
		l.Arrangement != "remote" || l.PostedAt == nil || l.PostedAt.UTC().Hour() != 17 {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
		return err
	})
}

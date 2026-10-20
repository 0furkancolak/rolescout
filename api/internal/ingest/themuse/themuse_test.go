package themuse

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	if a.Validate(map[string]any{"category": "<script>"}) == nil {
		t.Error("bad category accepted")
	}
	if err := a.Validate(map[string]any{"category": "Software Engineering", "location": "London, United Kingdom"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchReadsPagesAndFilters(t *testing.T) {
	var queries []string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Write([]byte(`{"page_count":2,"results":[{"id":18256005,"name":"Account Executive","contents":"<p>Sell</p>","publication_date":"2025-03-19T23:37:39Z","locations":[{"name":"Flexible / Remote"}],"levels":[{"name":"Mid Level"}],"refs":{"landing_page":"https://www.themuse.com/jobs/celonis/account-executive"},"company":{"name":"Celonis"}}]}`))
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"category": "Software Engineering"})
	if err == nil || len(got) != 2 || len(queries) != 2 {
		t.Fatalf("got %d listings in %d requests, err %v", len(got), len(queries), err)
	}
	if queries[0] != "category=Software+Engineering&page=1" {
		t.Errorf("query = %s", queries[0])
	}
	l := got[0]
	if l.ExternalID != "18256005" || l.Company != "Celonis" || l.Title != "Account Executive" ||
		l.Arrangement != "remote" || l.URL != "https://www.themuse.com/jobs/celonis/account-executive" || l.PostedAt == nil {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
		return err
	})
}

package himalayas

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	if a.Validate(map[string]any{"max_pages": "abc"}) == nil {
		t.Error("bad max_pages accepted")
	}
	if err := a.Validate(map[string]any{}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchPagesWithTheCursor(t *testing.T) {
	var cursors []string
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		next := `"nextCursor":"abc",`
		if r.URL.Query().Get("cursor") == "abc" {
			next = `"nextCursor":"",`
		}
		w.Write([]byte(`{` + next + `"jobs":[{"title":"Full-Stack Engineer","companyName":"thinkbridge","employmentType":"Full Time","locationRestrictions":["United States","Canada"],"description":"<p>Go</p>","excerpt":"x","pubDate":1792525425,"guid":"https://himalayas.app/companies/thinkbridge/jobs/` + r.URL.Query().Get("cursor") + `fs"}]}`))
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
	if err == nil || len(got) != 2 || len(cursors) != 2 || cursors[0] != "" || cursors[1] != "abc" {
		t.Fatalf("got %d listings, cursors %v, err %v", len(got), cursors, err)
	}
	l := got[0]
	if l.Title != "Full-Stack Engineer" || l.Company != "thinkbridge" || l.Location != "United States, Canada" ||
		l.Arrangement != "remote" || l.EmploymentType != "Full Time" || l.PostedAt == nil ||
		l.URL != "https://himalayas.app/companies/thinkbridge/jobs/fs" {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
		return err
	})
}

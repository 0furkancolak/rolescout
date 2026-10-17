package workday

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

func TestValidate(t *testing.T) {
	a := New(nil)
	for _, bad := range []map[string]any{
		{}, {"host": "evil.example.com", "tenant": "x", "site": "y"},
		{"host": "nvidia.wd5.myworkdayjobs.com", "tenant": "", "site": "y"},
		{"host": "nvidia.wd5.myworkdayjobs.com", "tenant": "nvidia", "site": "a/b"},
	} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v accepted", bad)
		}
	}
	if err := a.Validate(map[string]any{"host": "nvidia.wd5.myworkdayjobs.com", "tenant": "nvidia", "site": "NVIDIAExternalCareerSite"}); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMapsFixtureAndPages(t *testing.T) {
	var offsets []int
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/wday/cxs/nvidia/External/jobs" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		var req struct{ Limit, Offset int }
		json.NewDecoder(r.Body).Decode(&req)
		offsets = append(offsets, req.Offset)
		if req.Offset == 0 {
			ingesttest.File(t, "testdata/jobs.json")(w, r)
			return
		}
		fmt.Fprint(w, `{"total":3,"jobPostings":[]}`)
	})
	a := &Adapter{Client: c, BaseURL: base, PageSize: 2, Now: func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) }}
	got, err := a.Fetch(ingesttest.Ctx(t), map[string]any{"host": "nvidia.wd5.myworkdayjobs.com", "tenant": "nvidia", "site": "External", "company": "NVIDIA"})
	if err != nil {
		t.Fatal(err)
	}
	if len(offsets) != 2 || offsets[1] != 2 {
		t.Errorf("offsets = %v, want [0 2]", offsets)
	}
	if len(got) != 3 {
		t.Fatalf("listings = %d", len(got))
	}
	l := got[0]
	if l.ExternalID != "JR2024421" || l.URL != base+"/External/job/US-CA-Santa-Clara/Principal-Block-File-Storage-Software-Engineer--Linux---DGX-Cloud_JR2024421" ||
		l.Company != "NVIDIA" || l.Location != "US, CA, Santa Clara" || l.PostedAt == nil || l.PostedAt.Day() != 16 {
		t.Errorf("listing = %+v", l)
	}
	if got[1].Location != "US CA Santa Clara" {
		t.Errorf("multi-location job location = %q, want the primary location from its path", got[1].Location)
	}
}

func TestParsePostedOn(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]int{"Posted Today": 0, "Posted Yesterday": 1, "Posted 3 Days Ago": 3, "Posted 30+ Days Ago": 30} {
		got := postedOn(in, now)
		if got == nil || int(now.Sub(*got).Hours()/24) != want {
			t.Errorf("postedOn(%q) = %v, want %d days ago", in, got, want)
		}
	}
	if postedOn("whenever", now) != nil {
		t.Error("unknown text must give nil")
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{"host": "x.wd1.myworkdayjobs.com", "tenant": "x", "site": "y"})
		return err
	})
}

package remoteok

import (
	"net/http"
	"testing"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest/ingesttest"
)

const fixture = `[{"last_updated":1792483205,"legal":"API Terms of Service: link back"},
{"slug":"remote-support-1137547","id":"1137547","epoch":1792425604,"company":"SupportYourApp","position":"Customer Support Consultant","tags":["dev"],"description":"<p>Help customers</p>","location":"","url":"https://remoteOK.com/remote-jobs/remote-support-1137547"}]`

func TestValidate(t *testing.T) {
	if err := New(nil).Validate(map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if New(nil).Validate(map[string]any{"x": "y"}) == nil {
		t.Error("unknown field accepted")
	}
}

func TestFetchSkipsTheLegalNotice(t *testing.T) {
	c, base := ingesttest.Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(fixture))
	})
	got, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
	if err == nil || len(got) != 1 {
		t.Fatalf("got %d listings, err %v", len(got), err)
	}
	l := got[0]
	if l.ExternalID != "1137547" || l.Title != "Customer Support Consultant" || l.Company != "SupportYourApp" ||
		l.URL != "https://remoteOK.com/remote-jobs/remote-support-1137547" || l.Arrangement != "remote" ||
		l.PostedAt == nil || l.PostedAt.Year() != 2026 {
		t.Errorf("listing = %+v", l)
	}
}

func TestFetchErrors(t *testing.T) {
	ingesttest.ExpectNotFoundAndRetryable(t, func(c *crawl.Client, base string) error {
		_, err := (&Adapter{Client: c, BaseURL: base}).Fetch(ingesttest.Ctx(t), map[string]any{})
		return err
	})
}

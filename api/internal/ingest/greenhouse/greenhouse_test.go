package greenhouse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

const jobsJSON = `{"jobs":[{"id":127817,"title":"Senior Go Engineer","updated_at":"2026-10-15T17:21:31-05:00",
"first_published":"2026-10-01T09:00:00-05:00","absolute_url":"https://boards.greenhouse.io/acme/jobs/127817",
"content":"&lt;p&gt;Build things&lt;/p&gt;","location":{"name":"Remote"},"metadata":null}],"meta":{"total":1}}`

func server(t *testing.T, handler http.HandlerFunc) *Adapter {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	c := crawl.New(crawl.Options{AllowPrivate: true, UserAgent: "test-agent", HostInterval: time.Millisecond})
	return &Adapter{BaseURL: srv.URL, Client: c}
}

func TestFetchReadsBoardAndJobs(t *testing.T) {
	a := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("missing user agent")
		}
		switch r.URL.RequestURI() {
		case "/v1/boards/acme":
			w.Write([]byte(`{"name":"Acme Inc","content":""}`))
		case "/v1/boards/acme/jobs?content=true":
			w.Write([]byte(jobsJSON))
		default:
			http.NotFound(w, r)
		}
	})
	got, err := a.Fetch(context.Background(), map[string]any{"board_token": "acme"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d listings", len(got))
	}
	l := got[0]
	if l.ExternalID != "127817" || l.Company != "Acme Inc" || l.Location != "Remote" || l.Title != "Senior Go Engineer" {
		t.Fatalf("unexpected listing: %+v", l)
	}
	if l.SourceUpdatedAt == nil || !l.SourceUpdatedAt.Equal(time.Date(2026, 10, 15, 22, 21, 31, 0, time.UTC)) || l.SourceUpdatedAt.Location() != time.UTC {
		t.Fatalf("updated_at not parsed as UTC: %v", l.SourceUpdatedAt)
	}
	if l.PostedAt == nil {
		t.Fatal("first_published not parsed")
	}
	if text := ingest.HTMLToText(l.Description); text != "Build things" {
		t.Fatalf("description = %q", text)
	}
}

func TestFetchUsesConfiguredCompanyWithoutBoardRequest(t *testing.T) {
	a := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/boards/acme/jobs" {
			t.Errorf("unexpected request %s", r.URL)
		}
		w.Write([]byte(jobsJSON))
	})
	got, err := a.Fetch(context.Background(), map[string]any{"board_token": "acme", "company": "Acme"})
	if err != nil || got[0].Company != "Acme" {
		t.Fatalf("err=%v got=%+v", err, got)
	}
}

func TestFetchClassifiesErrors(t *testing.T) {
	cases := []struct {
		status    int
		retryable bool
	}{
		{http.StatusNotFound, false},
		{http.StatusForbidden, false},
		{http.StatusTooManyRequests, true},
		{http.StatusBadGateway, true},
	}
	for _, c := range cases {
		a := server(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(c.status) })
		_, err := a.Fetch(context.Background(), map[string]any{"board_token": "acme", "company": "Acme"})
		if err == nil || ingest.IsRetryable(err) != c.retryable {
			t.Errorf("status %d: err=%v retryable=%v, want %v", c.status, err, ingest.IsRetryable(err), c.retryable)
		}
	}
}

func TestFetchTreatsTimeoutsAsRetryable(t *testing.T) {
	var hits atomic.Int32
	a := server(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := a.Fetch(ctx, map[string]any{"board_token": "acme", "company": "Acme"})
	if !ingest.IsRetryable(err) || hits.Load() != 1 {
		t.Fatalf("err=%v hits=%d", err, hits.Load())
	}
}

func TestFetchRejectsMalformedJSON(t *testing.T) {
	a := server(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"jobs":`)) })
	_, err := a.Fetch(context.Background(), map[string]any{"board_token": "acme", "company": "Acme"})
	if err == nil || ingest.IsRetryable(err) || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("err=%v", err)
	}
}

func TestValidate(t *testing.T) {
	a := New(crawl.New(crawl.Options{AllowPrivate: true, UserAgent: "ua"}))
	for _, bad := range []map[string]any{
		{},
		{"board_token": ""},
		{"board_token": "../admin"},
		{"board_token": "acme", "company": 5},
		{"board_token": "acme", "url": "https://evil.example"},
	} {
		if a.Validate(bad) == nil {
			t.Errorf("config %v must be rejected", bad)
		}
	}
	if err := a.Validate(map[string]any{"board_token": "acme-2", "company": "Acme"}); err != nil {
		t.Fatal(err)
	}
}

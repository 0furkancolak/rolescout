// Package ingesttest has helpers for adapter tests.
package ingesttest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"example.com/rolescout/api/internal/crawl"
	"example.com/rolescout/api/internal/ingest"
)

// Server starts handler behind a 404 robots.txt and returns a crawl client
// for it and its base URL.
func Server(t *testing.T, handler http.HandlerFunc) (*crawl.Client, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return crawl.New(crawl.Options{UserAgent: "RoleScout/test", HostInterval: time.Millisecond}), srv.URL
}

// File serves the contents of a testdata file.
func File(t *testing.T, path string) http.HandlerFunc {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return func(w http.ResponseWriter, r *http.Request) { w.Write(body) }
}

// Status answers every request with code.
func Status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

// ExpectNotFoundAndRetryable checks the shared error mapping of an adapter:
// 404 is ErrBoardNotFound and not retryable, 503 is retryable.
func ExpectNotFoundAndRetryable(t *testing.T, fetch func(c *crawl.Client, baseURL string) error) {
	t.Helper()
	c, base := Server(t, Status(http.StatusNotFound))
	err := fetch(c, base)
	if !errors.Is(err, ingest.ErrBoardNotFound) || ingest.IsRetryable(err) {
		t.Errorf("404: err = %v, want ErrBoardNotFound", err)
	}
	c, base = Server(t, Status(http.StatusServiceUnavailable))
	if err := fetch(c, base); !ingest.IsRetryable(err) {
		t.Errorf("503: err = %v, want retryable", err)
	}
}

// Ctx is a short-lived context for adapter tests.
func Ctx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"example.com/rolescout/api/internal/infrastructure/database"
	"example.com/rolescout/api/internal/infrastructure/database/dbtest"
)

// testDB points DATABASE_URL at a fresh, migrated database.
func testDB(t *testing.T) {
	t.Helper()
	admin := os.Getenv(dbtest.EnvVar)
	if admin == "" {
		t.Skipf("%s is not set", dbtest.EnvVar)
	}
	url, drop, err := dbtest.Create(admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(drop)
	ctx := context.Background()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.RunMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", url)
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(context.Background(), args, &out)
	return out.String(), err
}

func TestAdaptersCommands(t *testing.T) {
	testDB(t)
	out, err := runCLI(t, "adapters", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "greenhouse") || !regexpLine(out, "workday", "no") {
		t.Fatalf("list output:\n%s", out)
	}
	if _, err := runCLI(t, "adapters", "enable", "-adapter", "workday"); err == nil {
		t.Fatal("enabling an unreviewed adapter must fail")
	}
	if _, err := runCLI(t, "adapters", "review", "-adapter", "workday", "-note", "cxs endpoint checked"); err != nil {
		t.Fatal(err)
	}
	if out, err = runCLI(t, "adapters", "enable", "-adapter", "workday"); err != nil || !strings.Contains(out, "enabled") {
		t.Fatalf("enable: %q %v", out, err)
	}
	if out, err = runCLI(t, "adapters", "disable", "-adapter", "workday"); err != nil || !strings.Contains(out, "disabled") {
		t.Fatalf("disable: %q %v", out, err)
	}
}

func TestCompaniesCommands(t *testing.T) {
	testDB(t)
	if _, err := runCLI(t, "companies", "add", "-url", "not-a-url"); err == nil {
		t.Fatal("invalid URL accepted")
	}
	out, err := runCLI(t, "companies", "add", "-url", "https://acme.example.com/careers", "-name", "Acme")
	if err != nil || !strings.Contains(out, "pending") {
		t.Fatalf("add: %q %v", out, err)
	}
	out, err = runCLI(t, "companies", "list", "-status", "pending")
	if err != nil || !strings.Contains(out, "Acme") || !strings.Contains(out, "https://acme.example.com/careers") {
		t.Fatalf("list:\n%s %v", out, err)
	}
}

// regexpLine reports whether a line contains both words.
func regexpLine(out, a, b string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, a) && strings.Contains(line, b) {
			return true
		}
	}
	return false
}

func TestSeedsCheckReportsDeadAndOK(t *testing.T) {
	var emptyHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/careers":
			w.Write([]byte(`<html><script type="application/ld+json">{"@type":"JobPosting","title":"Dev","url":"` + "http://" + r.Host + `/careers/1","hiringOrganization":"Acme"}</script></html>`))
		case "/empty":
			// Detection sees a posting; by the time jobs are read it is gone.
			if emptyHits.Add(1) == 1 {
				w.Write([]byte(`<html><script type="application/ld+json">{"@type":"JobPosting","title":"Last"}</script></html>`))
				return
			}
			w.Write([]byte(`<html></html>`))
		case "/plain":
			w.Write([]byte(`<html>Send us your CV</html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	in := filepath.Join(dir, "seeds.txt")
	out := filepath.Join(dir, "good.txt")
	os.WriteFile(in, []byte("# test\n"+srv.URL+"/careers | Acme\n"+srv.URL+"/plain | Plain\n"+srv.URL+"/gone | Gone\n"+srv.URL+"/empty | Empty\n"), 0o600)
	t.Setenv("DATABASE_URL", "postgres://unused")
	report, err := runCLI(t, "seeds", "check", "-file", in, "-write", out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "OK   jsonld") || !strings.Contains(report, "DEAD") || !strings.Contains(report, "1 of 4") {
		t.Fatalf("report:\n%s", report)
	}
	good, _ := os.ReadFile(out)
	if string(good) != srv.URL+"/careers | Acme\n" {
		t.Errorf("written seeds = %q", good)
	}
}

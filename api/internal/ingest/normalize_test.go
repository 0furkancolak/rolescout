package ingest

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestHTMLToTextUnescapesGreenhouseContent(t *testing.T) {
	in := `&lt;p&gt;We build &amp;amp; ship.&lt;/p&gt;&lt;ul&gt;&lt;li&gt;Go&lt;/li&gt;&lt;li&gt;PostgreSQL&lt;/li&gt;&lt;/ul&gt;&lt;script&gt;alert(1)&lt;/script&gt;&lt;p&gt;Apply   now&lt;/p&gt;`
	want := "We build & ship.\n\n• Go\n\n• PostgreSQL\n\nApply now"
	if got := HTMLToText(in); got != want {
		t.Fatalf("HTMLToText:\n got %q\nwant %q", got, want)
	}
}

func TestHTMLToTextDropsMarkupInPlainHTML(t *testing.T) {
	got := HTMLToText(`<div>Hello<br>world<style>p{}</style><img src=x onerror=alert(1)></div>`)
	if got != "Hello\nworld" {
		t.Fatalf("got %q", got)
	}
}

func TestDedupeKeyIgnoresCaseSpacingAndPunctuation(t *testing.T) {
	a := DedupeKey("Acme, Inc.", "Senior  Backend Engineer", "Istanbul")
	b := DedupeKey("acme inc", "senior backend engineer!", " istanbul ")
	if a != b {
		t.Fatal("equivalent listings must share a dedupe key")
	}
	if a == DedupeKey("Acme Inc", "Senior Backend Engineer", "Ankara") {
		t.Fatal("different locations must not share a dedupe key")
	}
	if !strings.HasPrefix(a, "v1:") {
		t.Fatalf("key %q must be versioned", a)
	}
}

func TestNormalizeInfersAttributes(t *testing.T) {
	seen := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	got, err := Normalize("src", RawListing{
		ExternalID:  " 42 ",
		URL:         "https://boards.example.com/acme/jobs/42",
		Title:       "Senior Go Engineer (Remote)",
		Company:     "Acme",
		Location:    "Istanbul, Turkey",
		Description: "<p>We use PostgreSQL, Kubernetes and some JavaScript. Let's go!</p>",
	}, seen)
	if err != nil {
		t.Fatal(err)
	}
	if got.ExternalID != "42" || got.WorkArrangement != "remote" || got.Seniority != "senior" || got.EmploymentType != "unknown" {
		t.Fatalf("unexpected attributes: %+v", got)
	}
	want := []string{"go", "javascript", "postgresql", "kubernetes"}
	if !reflect.DeepEqual(got.Technologies, want) {
		t.Fatalf("technologies = %v, want %v", got.Technologies, want)
	}
	if got.Location == nil || *got.Location != "Istanbul, Turkey" || !got.SeenAt.Equal(seen) {
		t.Fatalf("location/seenAt not kept: %+v", got)
	}
}

func TestTechnologyMatchingUsesWordBoundaries(t *testing.T) {
	got := technologies("frontend developer", "javascript and typescript; we go to conferences")
	want := []string{"javascript", "typescript"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v (no java, no go outside the title)", got, want)
	}
}

func TestSeniorityAndEmploymentType(t *testing.T) {
	cases := []struct{ title, seniority, employment string }{
		{"Software Engineering Intern", "intern", "internship"},
		{"Staff Engineer", "lead", "unknown"},
		{"Jr. Data Analyst - Part-time", "junior", "part_time"},
		{"Backend Developer (Contract)", "unknown", "contract"},
		{"Kıdemli Yazılım Geliştirici", "senior", "unknown"},
	}
	for _, c := range cases {
		if s := seniority(c.title); s != c.seniority {
			t.Errorf("seniority(%q) = %s, want %s", c.title, s, c.seniority)
		}
		if e := employmentType("", c.title); e != c.employment {
			t.Errorf("employmentType(%q) = %s, want %s", c.title, e, c.employment)
		}
	}
}

func TestNormalizeRejectsInvalidListings(t *testing.T) {
	_, err := Normalize("src", RawListing{ExternalID: "1", Title: " ", Company: "Acme", URL: "javascript:alert(1)"}, time.Now())
	if !errors.Is(err, ErrInvalidListing) {
		t.Fatalf("error = %v, want ErrInvalidListing", err)
	}
	for _, part := range []string{"title is empty", "url must be"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not mention %q", err, part)
		}
	}
}

func TestTruncateKeepsValidUTF8(t *testing.T) {
	s := strings.Repeat("ş", 10) // 2 bytes each
	got := truncate(s, 5)
	if !utf8.ValidString(got) || got != "şş" {
		t.Fatalf("truncate = %q", got)
	}
}

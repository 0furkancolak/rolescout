package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/rolescout/api/internal/llm"
	"example.com/rolescout/api/internal/llm/llmtest"
)

func ip(n int) *int { return &n }

var cases = []Case{
	{ID: "en-1", Lang: "en", Title: "Senior Go Developer", Company: "A", Description: "Senior Go Developer wanted. We use Go, PostgreSQL and Docker. Remote within the EU. Salary 90,000 - 110,000 EUR per year.",
		Expect: Expect{Seniority: "senior", WorkArrangement: "remote", Technologies: []string{"go", "postgresql", "docker"}, SalaryMin: ip(90000), SalaryMax: ip(110000), SalaryCurrency: "EUR", SalaryPeriod: "year"}},
	{ID: "tr-1", Lang: "tr", Title: "Yazılım Stajyeri", Company: "B", Description: "Yazılım stajyeri arıyoruz. React ve Node.js kullanılır. İstanbul ofisinde çalışılır.",
		Expect: Expect{Seniority: "intern", WorkArrangement: "onsite", CountryCode: "TR", Technologies: []string{"react", "node.js"}}},
	{ID: "en-2", Lang: "en", Title: "Backend Engineer", Company: "C", Description: "Backend engineer for a payments team. We use Java and Kafka. On-site in Dublin.",
		Expect: Expect{Seniority: "unknown", WorkArrangement: "onsite", CountryCode: "IE", Technologies: []string{"java", "kafka"}}},
}

const (
	reply1 = `{"technologies":["go","postgresql","docker"],"seniority":"senior","work_arrangement":"remote","remote_policy":"","employment_type":"unknown","salary_min":90000,"salary_max":110000,"salary_currency":"EUR","salary_period":"year","location":"","country_code":"","languages":[],"years_experience":null,"summary":""}`
	// wrong arrangement, one invented technology, no salary needed
	reply2 = `{"technologies":["react","node.js","rust"],"seniority":"intern","work_arrangement":"hybrid","remote_policy":"","employment_type":"internship","salary_min":null,"salary_max":null,"salary_currency":"","salary_period":"unknown","location":"İstanbul","country_code":"TR","languages":[],"years_experience":null,"summary":""}`
	// invalid twice -> counted as an invalid output
	bad = `nope`
)

func TestRunComputesTheMetrics(t *testing.T) {
	srv := llmtest.New(t)
	srv.Script(llmtest.Scripted{Content: reply1, PromptTokens: 300, CompletionTokens: 90}, llmtest.Scripted{Content: reply2, PromptTokens: 300, CompletionTokens: 90},
		llmtest.Scripted{Content: bad}, llmtest.Scripted{Content: bad})
	c := llm.NewClient(llm.Options{BaseURL: srv.BaseURL(), Model: "m", Timeout: 2 * time.Second})
	rep := Run(context.Background(), c, cases, Options{Model: "m", MaxChars: 6000, MaxTokens: 400, ResolveCountry: func(p string) string {
		return map[string]string{"İstanbul": "TR", "Dublin": "IE"}[p]
	}})

	if rep.Cases != 3 || rep.Errors != 1 {
		t.Fatalf("cases %d errors %d", rep.Cases, rep.Errors)
	}
	// 2 of 3 produced a usable answer after validation; the first attempt was
	// valid JSON for 2 of 3 as well.
	if !near(rep.SchemaValidRate, 2.0/3) || !near(rep.ValidJSONRate, 2.0/3) {
		t.Errorf("schema-valid %.3f valid-json %.3f", rep.SchemaValidRate, rep.ValidJSONRate)
	}
	// Over the two answered cases: seniority both right; arrangement 1 of 2.
	if !near(rep.Seniority, 1) || !near(rep.WorkArrangement, 0.5) {
		t.Errorf("seniority %.2f arrangement %.2f", rep.Seniority, rep.WorkArrangement)
	}
	// Technologies kept: case 1 three right; case 2 two right ("rust" dropped as ungrounded).
	if !near(rep.TechPrecision, 1) || !near(rep.TechRecall, 1) {
		t.Errorf("tech precision %.2f recall %.2f", rep.TechPrecision, rep.TechRecall)
	}
	// 1 of 6 returned technologies was dropped as ungrounded.
	if !near(rep.GroundedDropRate, 1.0/6) {
		t.Errorf("grounded drop rate %.3f, want 1/6", rep.GroundedDropRate)
	}
	// Salary: case 1 exact, case 2 expected none and got none: 2 of 2.
	if !near(rep.SalaryExact, 1) {
		t.Errorf("salary exact %.2f", rep.SalaryExact)
	}
	if !near(rep.Country, 1) {
		t.Errorf("country %.2f", rep.Country)
	}
	if rep.LatencyMedian <= 0 || rep.LatencyP95 < rep.LatencyMedian {
		t.Errorf("latency median %s p95 %s", rep.LatencyMedian, rep.LatencyP95)
	}
	out := rep.Format()
	for _, want := range []string{"cases", "valid JSON", "schema-valid", "seniority", "technologies precision", "salary exact", "median", "p95"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func near(a, b float64) bool { d := a - b; return d > -0.01 && d < 0.01 }

func TestLoadCasesReadsAJSONFileOrDirectory(t *testing.T) {
	dir := t.TempDir()
	body := `[{"id":"x","lang":"en","title":"T","company":"C","description":"D","expect":{"seniority":"unknown","work_arrangement":"unknown","technologies":[]}}]`
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCases(dir)
	if err != nil || len(got) != 1 || got[0].ID != "x" {
		t.Fatalf("cases = %+v, err = %v", got, err)
	}
	if _, err := LoadCases(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing path must be an error")
	}
}

func TestTheShippedEvaluationSetIsWellFormed(t *testing.T) {
	cs, err := LoadCases("../testdata/eval")
	if err != nil {
		t.Fatal(err)
	}
	en, tr := 0, 0
	ids := map[string]bool{}
	for _, c := range cs {
		if ids[c.ID] || c.Title == "" || len(c.Description) < 100 {
			t.Fatalf("bad case %+v", c.ID)
		}
		ids[c.ID] = true
		switch c.Lang {
		case "en":
			en++
		case "tr":
			tr++
		default:
			t.Fatalf("case %s has language %q", c.ID, c.Lang)
		}
	}
	if en != 30 || tr != 30 {
		t.Fatalf("english %d turkish %d, want 30 each", en, tr)
	}
}

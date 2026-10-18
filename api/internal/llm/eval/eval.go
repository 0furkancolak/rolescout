// Package eval measures a model against a labelled set of job descriptions:
// how often it returns something usable, how accurate the validated fields
// are, how many invented technologies the grounding check had to drop, and how
// long it takes. It backs "admin llm eval" and the numbers in ADR 0005.
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"example.com/rolescout/api/internal/llm"
)

// Expect is what the labelled posting really says.
type Expect struct {
	Seniority       string   `json:"seniority"`
	WorkArrangement string   `json:"work_arrangement"`
	CountryCode     string   `json:"country_code"`
	Technologies    []string `json:"technologies"`
	SalaryMin       *int     `json:"salary_min"`
	SalaryMax       *int     `json:"salary_max"`
	SalaryCurrency  string   `json:"salary_currency"`
	SalaryPeriod    string   `json:"salary_period"`
}

// Case is one labelled posting.
type Case struct {
	ID          string `json:"id"`
	Lang        string `json:"lang"`
	Title       string `json:"title"`
	Company     string `json:"company"`
	Description string `json:"description"`
	Expect      Expect `json:"expect"`
}

// Options configures a run.
type Options struct {
	Model          string
	MaxChars       int
	MaxTokens      int
	ResolveCountry func(place string) string
	// Progress, when set, is called after each case.
	Progress func(done, total int, c Case, err error, took time.Duration)
}

// Report is the outcome of a run. Rates are fractions between 0 and 1.
type Report struct {
	Model string
	Cases int
	// Errors counts cases with no usable answer (transport error or invalid
	// output twice).
	Errors int
	// ValidJSONRate: the first reply parsed. SchemaValidRate: an answer remained
	// after validation (retries allowed).
	ValidJSONRate   float64
	SchemaValidRate float64
	// Field accuracy over the answered cases.
	Seniority       float64
	WorkArrangement float64
	Country         float64
	SalaryExact     float64
	// Technologies, micro-averaged over answered cases.
	TechPrecision float64
	TechRecall    float64
	// GroundedDropRate: share of returned technologies that were not in the text.
	GroundedDropRate float64
	LatencyMedian    time.Duration
	LatencyP95       time.Duration
	ByLanguage       map[string]LangReport
}

// LangReport splits the headline numbers by language.
type LangReport struct {
	Cases         int
	SchemaValid   float64
	Seniority     float64
	TechPrecision float64
	TechRecall    float64
}

// LoadCases reads a JSON file (an array of cases) or every *.json in a directory.
func LoadCases(path string) ([]Case, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	files := []string{path}
	if info.IsDir() {
		files, err = filepath.Glob(filepath.Join(path, "*.json"))
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
	}
	var out []Case
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var cs []Case
		if err := json.Unmarshal(raw, &cs); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out = append(out, cs...)
	}
	return out, nil
}

type tally struct {
	cases, answered, firstTry               int
	seniority, arrangement, country, salary int
	techTP, techReturned, techExpected      int
	dropped, techBeforeValidation           int
}

func (t *tally) add(o tally) {
	t.cases += o.cases
	t.answered += o.answered
	t.firstTry += o.firstTry
	t.seniority += o.seniority
	t.arrangement += o.arrangement
	t.country += o.country
	t.salary += o.salary
	t.techTP += o.techTP
	t.techReturned += o.techReturned
	t.techExpected += o.techExpected
	t.dropped += o.dropped
	t.techBeforeValidation += o.techBeforeValidation
}

func rate(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// Run sends every case through llm.Extract and scores the validated result.
func Run(ctx context.Context, c llm.Completer, cases []Case, o Options) Report {
	var total tally
	byLang := map[string]*tally{}
	var latencies []time.Duration
	errors := 0
	for i, cs := range cases {
		start := time.Now()
		got, stats, err := llm.Extract(ctx, c, llm.Input{
			Model: o.Model, Title: cs.Title, Company: cs.Company, Description: cs.Description,
			MaxChars: o.MaxChars, MaxTokens: o.MaxTokens, ResolveCountry: o.ResolveCountry,
		})
		took := time.Since(start)
		t := tally{cases: 1}
		if err != nil {
			errors++
		} else {
			t.answered = 1
			if stats.Retries == 0 {
				t.firstTry = 1
			}
			latencies = append(latencies, took)
			score(&t, cs.Expect, got, stats)
		}
		total.add(t)
		lt := byLang[cs.Lang]
		if lt == nil {
			lt = &tally{}
			byLang[cs.Lang] = lt
		}
		lt.add(t)
		if o.Progress != nil {
			o.Progress(i+1, len(cases), cs, err, took)
		}
	}

	rep := Report{
		Model: o.Model, Cases: len(cases), Errors: errors,
		ValidJSONRate: rate(total.firstTry, total.cases), SchemaValidRate: rate(total.answered, total.cases),
		Seniority: rate(total.seniority, total.answered), WorkArrangement: rate(total.arrangement, total.answered),
		Country: rate(total.country, total.answered), SalaryExact: rate(total.salary, total.answered),
		TechPrecision: rate(total.techTP, total.techReturned), TechRecall: rate(total.techTP, total.techExpected),
		GroundedDropRate: rate(total.dropped, total.techBeforeValidation),
		ByLanguage:       map[string]LangReport{},
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	if n := len(latencies); n > 0 {
		rep.LatencyMedian = latencies[n/2]
		rep.LatencyP95 = latencies[min(n-1, int(float64(n)*0.95))]
	}
	for lang, t := range byLang {
		rep.ByLanguage[lang] = LangReport{
			Cases: t.cases, SchemaValid: rate(t.answered, t.cases), Seniority: rate(t.seniority, t.answered),
			TechPrecision: rate(t.techTP, t.techReturned), TechRecall: rate(t.techTP, t.techExpected),
		}
	}
	return rep
}

func score(t *tally, want Expect, got llm.Enrichment, stats llm.Stats) {
	if got.Seniority == orUnknown(want.Seniority) {
		t.seniority = 1
	}
	if got.WorkArrangement == orUnknown(want.WorkArrangement) {
		t.arrangement = 1
	}
	if got.CountryCode == want.CountryCode {
		t.country = 1
	}
	if salaryEqual(got, want) {
		t.salary = 1
	}
	for _, tech := range got.Technologies {
		if slices.Contains(want.Technologies, tech) {
			t.techTP++
		}
	}
	t.techReturned = len(got.Technologies)
	t.techExpected = len(want.Technologies)
	t.dropped = stats.Dropped.Technologies
	t.techBeforeValidation = len(got.Technologies) + stats.Dropped.Technologies
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func salaryEqual(got llm.Enrichment, want Expect) bool {
	eq := func(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
	if want.SalaryMin == nil && want.SalaryMax == nil {
		return got.SalaryMin == nil && got.SalaryMax == nil
	}
	return eq(got.SalaryMin, want.SalaryMin) && eq(got.SalaryMax, want.SalaryMax) &&
		got.SalaryCurrency == want.SalaryCurrency && got.SalaryPeriod == want.SalaryPeriod
}

// Format renders the report as plain text.
func (r Report) Format() string {
	var b strings.Builder
	pct := func(v float64) string { return fmt.Sprintf("%5.1f%%", v*100) }
	fmt.Fprintf(&b, "model: %s\n", r.Model)
	fmt.Fprintf(&b, "cases: %d (no usable answer: %d)\n", r.Cases, r.Errors)
	fmt.Fprintf(&b, "valid JSON on the first reply:    %s\n", pct(r.ValidJSONRate))
	fmt.Fprintf(&b, "schema-valid after validation:    %s\n", pct(r.SchemaValidRate))
	fmt.Fprintf(&b, "seniority accuracy:               %s\n", pct(r.Seniority))
	fmt.Fprintf(&b, "work arrangement accuracy:        %s\n", pct(r.WorkArrangement))
	fmt.Fprintf(&b, "country accuracy:                 %s\n", pct(r.Country))
	fmt.Fprintf(&b, "technologies precision:           %s\n", pct(r.TechPrecision))
	fmt.Fprintf(&b, "technologies recall:              %s\n", pct(r.TechRecall))
	fmt.Fprintf(&b, "salary exact:                     %s\n", pct(r.SalaryExact))
	fmt.Fprintf(&b, "dropped as ungrounded:            %s of returned technologies\n", pct(r.GroundedDropRate))
	fmt.Fprintf(&b, "latency median: %s   p95: %s\n", r.LatencyMedian.Round(10*time.Millisecond), r.LatencyP95.Round(10*time.Millisecond))
	langs := make([]string, 0, len(r.ByLanguage))
	for l := range r.ByLanguage {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	for _, l := range langs {
		lr := r.ByLanguage[l]
		fmt.Fprintf(&b, "  %s: %d cases, schema-valid %s, seniority %s, technologies P %s R %s\n",
			l, lr.Cases, pct(lr.SchemaValid), pct(lr.Seniority), pct(lr.TechPrecision), pct(lr.TechRecall))
	}
	return b.String()
}

package llm

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func ip(n int) *int { return &n }

func validate(e Enrichment, title, desc string) (Enrichment, Dropped) {
	return Validate(e, title, desc, resolve)
}

func TestValidateCoercesEnumsToUnknown(t *testing.T) {
	got, d := validate(Enrichment{Seniority: "principal-staff", WorkArrangement: "sometimes", EmploymentType: "gig"}, "Engineer", "text")
	if got.Seniority != "unknown" || got.WorkArrangement != "unknown" || got.EmploymentType != "unknown" || d.Enums != 3 {
		t.Fatalf("got %+v dropped %+v", got, d)
	}
}

func TestValidatePrincipalMapsToLeadWhenStated(t *testing.T) {
	got, _ := validate(Enrichment{Seniority: "principal"}, "Principal Engineer", "text")
	if got.Seniority != "lead" {
		t.Fatalf("seniority = %q, want lead (principal is a lead level)", got.Seniority)
	}
}

func TestValidateSeniorityNeedsACueInTheText(t *testing.T) {
	for _, tc := range []struct{ level, title, desc, want string }{
		{"senior", "Engineer", "We want a senior person.", "senior"},
		{"senior", "Engineer", "Build things.", "unknown"},
		{"senior", "Kıdemli Geliştirici", "Servis geliştirin.", "senior"},
		{"junior", "Developer", "Entry level role.", "junior"},
		{"intern", "Yazılım Stajyeri", "x", "intern"},
		{"mid", "Developer", "3 years of experience", "unknown"},
		{"mid", "Mid-level Developer", "x", "mid"},
		{"lead", "Tech Lead", "x", "lead"},
		{"unknown", "Senior Dev", "x", "unknown"},
	} {
		got, _ := validate(Enrichment{Seniority: tc.level}, tc.title, tc.desc)
		if got.Seniority != tc.want {
			t.Errorf("%s in %q / %q -> %q, want %q", tc.level, tc.title, tc.desc, got.Seniority, tc.want)
		}
	}
}

func TestValidateTechnologiesAreGrounded(t *testing.T) {
	desc := "We use Go, k8s, JS, Node.js, C++ and PostgreSQL. Golang services."
	got, d := validate(Enrichment{Technologies: []string{"Go", "Kubernetes", "javascript", "node.js", "C++", "postgresql", "Rust", "GO", "docker", " "}}, "t", desc)
	want := []string{"go", "kubernetes", "javascript", "node.js", "c++", "postgresql"}
	if !reflect.DeepEqual(got.Technologies, want) {
		t.Fatalf("technologies = %v, want %v", got.Technologies, want)
	}
	if d.Technologies != 2 {
		t.Fatalf("dropped = %d, want 2 (rust, docker)", d.Technologies)
	}
}

func TestValidateTechnologiesAreWholeWords(t *testing.T) {
	got, _ := validate(Enrichment{Technologies: []string{"go", "java", "r"}}, "t", "We love algorithms and javascript. Let's go further.")
	// "go" is a word in the text, "java" only appears inside "javascript", "r" is too short to trust.
	if !reflect.DeepEqual(got.Technologies, []string{"go"}) {
		t.Fatalf("technologies = %v", got.Technologies)
	}
}

func TestValidateTechnologiesCapped(t *testing.T) {
	var names, words []string
	for i := 0; i < 40; i++ {
		n := "tech" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		names = append(names, n)
		words = append(words, n)
	}
	got, _ := validate(Enrichment{Technologies: names}, "t", strings.Join(words, " "))
	if len(got.Technologies) != 25 {
		t.Fatalf("len = %d, want 25", len(got.Technologies))
	}
}

func TestValidateSalary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in       Enrichment
		desc     string
		min, max *int
		cur      string
	}{
		{"short k notation vs full numbers", Enrichment{SalaryMin: ip(100000), SalaryMax: ip(120000), SalaryCurrency: "USD", SalaryPeriod: "year"}, "Pay: $100,000 - $120,000", ip(100000), ip(120000), "USD"},
		{"k in text", Enrichment{SalaryMin: ip(100000), SalaryMax: ip(120000), SalaryCurrency: "USD", SalaryPeriod: "year"}, "Salary 100k-120k", ip(100000), ip(120000), "USD"},
		{"turkish thousands dots", Enrichment{SalaryMin: ip(120000), SalaryMax: ip(160000), SalaryCurrency: "TRY", SalaryPeriod: "month"}, "Maaş 120.000 - 160.000 TL", ip(120000), ip(160000), "TRY"},
		{"space separated", Enrichment{SalaryMin: ip(80000), SalaryCurrency: "EUR", SalaryPeriod: "year"}, "from 80 000 EUR", ip(80000), nil, "EUR"},
		{"not in text", Enrichment{SalaryMin: ip(900000), SalaryMax: ip(900000), SalaryCurrency: "USD", SalaryPeriod: "year"}, "Pay: $100,000", nil, nil, ""},
		{"min above max", Enrichment{SalaryMin: ip(120000), SalaryMax: ip(100000), SalaryCurrency: "USD", SalaryPeriod: "year"}, "$120,000 and $100,000", nil, nil, ""},
		{"implausible hourly", Enrichment{SalaryMin: ip(90000), SalaryCurrency: "USD", SalaryPeriod: "hour"}, "90,000", nil, nil, ""},
		{"unknown period", Enrichment{SalaryMin: ip(90000), SalaryCurrency: "USD", SalaryPeriod: "unknown"}, "90,000", nil, nil, ""},
		{"currency symbol and name", Enrichment{SalaryMin: ip(120000), SalaryMax: ip(160000), SalaryCurrency: "TL", SalaryPeriod: "month"}, "Maaş 120.000 - 160.000 TL", ip(120000), ip(160000), "TRY"},
		{"euro sign", Enrichment{SalaryMin: ip(90000), SalaryCurrency: "€", SalaryPeriod: "year"}, "from €90,000", ip(90000), nil, "EUR"},
		{"dollar sign", Enrichment{SalaryMin: ip(100000), SalaryCurrency: "$", SalaryPeriod: "year"}, "$100,000", ip(100000), nil, "USD"},
		{"bad currency", Enrichment{SalaryMin: ip(90000), SalaryCurrency: "moneys", SalaryPeriod: "year"}, "90,000", nil, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, d := validate(tc.in, "t", tc.desc)
			if !reflect.DeepEqual(got.SalaryMin, tc.min) || !reflect.DeepEqual(got.SalaryMax, tc.max) || got.SalaryCurrency != tc.cur {
				t.Fatalf("salary = %v-%v %q, want %v-%v %q", deref(got.SalaryMin), deref(got.SalaryMax), got.SalaryCurrency, deref(tc.min), deref(tc.max), tc.cur)
			}
			if tc.min == nil && !d.Salary {
				t.Fatalf("dropped = %+v, want Salary", d)
			}
		})
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestValidateCountryRulesWin(t *testing.T) {
	got, _ := validate(Enrichment{Location: "Berlin, Germany", CountryCode: "FR"}, "t", "x")
	if got.CountryCode != "DE" {
		t.Fatalf("country = %q, the rules say DE", got.CountryCode)
	}
	got, _ = validate(Enrichment{Location: "Nowhere Land", CountryCode: "tr"}, "t", "x")
	if got.CountryCode != "TR" {
		t.Fatalf("country = %q, want TR kept (upper-cased) when the rules do not know the place", got.CountryCode)
	}
	got, d := validate(Enrichment{Location: "", CountryCode: "Turkey"}, "t", "x")
	if got.CountryCode != "" || !d.Country {
		t.Fatalf("country = %q dropped = %+v, want an invalid code dropped", got.CountryCode, d)
	}
}

func TestValidateYearsLanguagesSummary(t *testing.T) {
	got, d := validate(Enrichment{YearsExperience: ip(55), Languages: []string{"EN", "tr", "turkish", "tr", "de", "fr", "es", "it"}}, "t", "x")
	if got.YearsExperience != nil || !d.Years {
		t.Fatalf("years = %v dropped %+v", got.YearsExperience, d)
	}
	if !reflect.DeepEqual(got.Languages, []string{"en", "tr", "de", "fr", "es"}) {
		t.Fatalf("languages = %v", got.Languages)
	}
	long := "  " + strings.Repeat("Çok güzel bir iş. ", 40) + "\x00\x07 end"
	got, _ = validate(Enrichment{Summary: long, RemotePolicy: strings.Repeat("r", 300)}, "t", "x")
	if utf8.RuneCountInString(got.Summary) > 300 || !utf8.ValidString(got.Summary) || strings.ContainsAny(got.Summary, "\x00\x07") || got.Summary != strings.TrimSpace(got.Summary) {
		t.Fatalf("summary = %q", got.Summary)
	}
	if utf8.RuneCountInString(got.RemotePolicy) > 120 {
		t.Fatalf("remote policy has %d runes", utf8.RuneCountInString(got.RemotePolicy))
	}
}

func TestValidateDropsPlaceholderTexts(t *testing.T) {
	got, _ := validate(Enrichment{RemotePolicy: "unknown", Location: "N/A", Summary: "  none "}, "t", "x")
	if got.RemotePolicy != "" || got.Location != "" || got.Summary != "" {
		t.Fatalf("placeholders kept: %+v", got)
	}
}

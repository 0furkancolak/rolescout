package llm

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Dropped counts what validation removed from a model reply.
type Dropped struct {
	Technologies int
	Enums        int
	Salary       bool
	Country      bool
	Years        bool
}

const maxTechnologies = 25

var urlPattern = regexp.MustCompile(`(?i)https?://|www\.|\b[a-z0-9-]+\.(com|net|org|io|co|app|dev)\b/?`)

var (
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	countryPattern  = regexp.MustCompile(`^[A-Z]{2}$`)
	languagePattern = regexp.MustCompile(`^[a-z]{2}$`)
)

// seniorityCues are words that must appear in the title or description for the
// model's seniority claim to stand. The model reading "3 years of experience"
// and answering "mid" is a guess, not something the posting states.
// seniorityTitleOnly are cues too common in prose ("you will lead", "our staff",
// "mid-sized") to trust outside the title.
var seniorityTitleOnly = map[string]bool{"mid": true, "lead": true, "junior": false}

var seniorityCues = map[string][]string{
	"intern": {"intern", "internship", "stajyer*", "staj", "trainee", "working student"},
	"junior": {"junior", "jr", "entry level", "entry-level", "graduate", "new grad", "yeni mezun"},
	"mid":    {"mid", "mid-level", "mid level", "intermediate", "medior"},
	"senior": {"senior", "sr", "kıdemli*", "experienced"},
	"lead":   {"lead", "principal", "staff", "head of", "architect", "takım lideri", "tech lead"},
}

// salaryBounds are sanity limits per period, in any currency; values outside
// are far more likely to be a misread number than a salary.
var salaryBounds = map[string][2]int{
	"year":  {5_000, 2_000_000},
	"month": {300, 200_000},
	"day":   {20, 5_000},
	"hour":  {3, 1_000},
}

// Validate keeps only what the text supports. resolveCountry may be nil.
func Validate(e Enrichment, title, description string, resolveCountry func(place string) string) (Enrichment, Dropped) {
	var d Dropped
	text := strings.ToLower(title + "\n" + description)

	coerce := func(v string, allowed []string) string {
		v = strings.ToLower(strings.TrimSpace(v))
		if slices.Contains(allowed, v) {
			return v
		}
		if v != "" {
			d.Enums++
		}
		return "unknown"
	}
	e.Seniority = coerce(e.Seniority, seniorityEnum)
	e.WorkArrangement = coerce(e.WorkArrangement, arrangementEnum)
	e.EmploymentType = coerce(e.EmploymentType, employmentEnum)
	e.SalaryPeriod = coerce(e.SalaryPeriod, salaryPeriodEnum)
	if e.Seniority == "principal" {
		e.Seniority = "lead"
	}
	if e.Seniority != "unknown" {
		scope := text
		if seniorityTitleOnly[e.Seniority] {
			scope = strings.ToLower(title)
		}
		if !hasCue(scope, seniorityCues[e.Seniority]) {
			e.Seniority = "unknown"
		}
	}

	if e.WorkArrangement != "unknown" && !hasCue(text, arrangementCues[e.WorkArrangement]) {
		e.WorkArrangement = "unknown"
		d.Enums++
	}

	e.Technologies = groundTechnologies(e.Technologies, text, title+"\n"+description, &d)

	e.SalaryMin, e.SalaryMax, e.SalaryCurrency, e.SalaryPeriod = validateSalary(e, title+"\n"+description, &d)

	e.Location = truncateRunes(placeholderToEmpty(cleanText(e.Location)), 120)
	e.CountryCode = strings.ToUpper(strings.TrimSpace(e.CountryCode))
	if resolveCountry != nil && e.Location != "" {
		if code := resolveCountry(e.Location); code != "" {
			e.CountryCode = code
		}
	}
	if e.CountryCode != "" && !countryPattern.MatchString(e.CountryCode) {
		e.CountryCode = ""
		d.Country = true
	}
	// A country the rules could not read from the location must be named in
	// the text, or the model made it up.
	if e.CountryCode != "" && resolveCountry != nil && resolveCountry(e.Location) != e.CountryCode && !mentionsCountry(title+"\n"+description, e.CountryCode, resolveCountry) {
		e.CountryCode = ""
		d.Country = true
	}

	if e.YearsExperience != nil && (*e.YearsExperience < 0 || *e.YearsExperience > 40) {
		e.YearsExperience = nil
		d.Years = true
	}

	e.Languages = cleanLanguages(e.Languages)
	e.RemotePolicy = truncateRunes(placeholderToEmpty(cleanText(e.RemotePolicy)), 120)
	e.Summary = truncateRunes(placeholderToEmpty(cleanText(e.Summary)), 300)
	if urlPattern.MatchString(e.Summary) {
		e.Summary = ""
	}
	if urlPattern.MatchString(e.RemotePolicy) {
		e.RemotePolicy = ""
	}
	return e, d
}

// hasCue looks for any cue as a whole word; a trailing "*" lets the word
// continue (Turkish suffixes: "stajyeri", "kıdemli").
func hasCue(text string, cues []string) bool {
	for _, c := range cues {
		if stem, ok := strings.CutSuffix(c, "*"); ok {
			if containsPrefixWord(text, stem) {
				return true
			}
		} else if containsWord(text, c) {
			return true
		}
	}
	return false
}

func groundTechnologies(in []string, text, original string, d *Dropped) []string {
	type item struct {
		name string
		ok   bool
	}
	var items []item
	seen := map[string]bool{}
	for _, raw := range in {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		items = append(items, item{name: name})
	}
	// Ordinary names first: an ambiguous one ("go") is also accepted next to
	// a technology that was found.
	var accepted []string
	for pass := 0; pass < 2; pass++ {
		for i := range items {
			it := &items[i]
			if ambiguousTech[it.name] != (pass == 1) {
				continue
			}
			if len([]rune(it.name)) < 2 || len(accepted) >= maxTechnologies || !grounded(it.name, text) {
				continue
			}
			if pass == 1 && !usedAsTechnology(it.name, original, accepted) {
				continue
			}
			it.ok = true
			accepted = append(accepted, it.name)
		}
	}
	var out []string
	for _, it := range items {
		if it.ok {
			out = append(out, it.name)
		} else {
			d.Technologies++
		}
	}
	return out
}

func grounded(name, text string) bool {
	if containsWord(text, name) {
		return true
	}
	for _, alias := range technologyAliases[name] {
		if containsWord(text, alias) {
			return true
		}
	}
	// "kubernetes" may be named by its alias in the reply instead of the text.
	for canon, aliases := range technologyAliases {
		if slices.Contains(aliases, name) && containsWord(text, canon) {
			return true
		}
	}
	return false
}

func validateSalary(e Enrichment, text string, d *Dropped) (min, max *int, currency, period string) {
	if e.SalaryMin == nil && e.SalaryMax == nil {
		return nil, nil, "", "unknown"
	}
	drop := func() (*int, *int, string, string) {
		d.Salary = true
		return nil, nil, "", "unknown"
	}
	currency = normalizeCurrency(e.SalaryCurrency)
	if !currencyPattern.MatchString(currency) || e.SalaryPeriod == "unknown" {
		return drop()
	}
	bounds := salaryBounds[e.SalaryPeriod]
	nums := numbersIn(text)
	for _, v := range []*int{e.SalaryMin, e.SalaryMax} {
		if v == nil {
			continue
		}
		if *v < bounds[0] || *v > bounds[1] || !nums[*v] {
			return drop()
		}
	}
	if e.SalaryMin != nil && e.SalaryMax != nil && *e.SalaryMin > *e.SalaryMax {
		return drop()
	}
	return e.SalaryMin, e.SalaryMax, currency, e.SalaryPeriod
}

func cleanLanguages(in []string) []string {
	var out []string
	for _, l := range in {
		l = strings.ToLower(strings.TrimSpace(l))
		if languagePattern.MatchString(l) && !slices.Contains(out, l) {
			out = append(out, l)
		}
		if len(out) == 5 {
			break
		}
	}
	return out
}

// currencyAliases maps symbols and common local names to ISO 4217 codes.
var currencyAliases = map[string]string{
	"$": "USD", "US$": "USD", "USD$": "USD", "€": "EUR", "£": "GBP", "¥": "JPY", "₺": "TRY", "TL": "TRY",
	"TÜRK LİRASI": "TRY", "TURKISH LIRA": "TRY", "DOLLAR": "USD", "DOLLARS": "USD", "EURO": "EUR", "EUROS": "EUR",
	"POUND": "GBP", "POUNDS": "GBP", "CHF": "CHF", "₹": "INR",
}

func normalizeCurrency(raw string) string {
	c := strings.ToUpper(strings.TrimSpace(raw))
	if iso, ok := currencyAliases[c]; ok {
		return iso
	}
	return c
}

// placeholderToEmpty turns the words a model writes for "nothing" into "".
func placeholderToEmpty(s string) string {
	switch strings.ToLower(strings.Trim(s, " .")) {
	case "unknown", "n/a", "na", "none", "null", "not stated", "not specified", "belirtilmemiş", "yok":
		return ""
	}
	return s
}

// arrangementCues are words that support each work arrangement.
var arrangementCues = map[string][]string{
	"remote": {"remote", "remotely", "uzaktan", "work from home", "wfh", "home office", "evden", "fully distributed"},
	"hybrid": {"hybrid", "hibrit", "melez"},
	"onsite": {"on-site", "onsite", "on site", "in-office", "in office", "office-based", "ofisten", "ofiste", "yerinde", "iş yerinde"},
}

// mentionsCountry reports whether any short stretch of text names a place in
// the country. resolve maps a place to its country code.
func mentionsCountry(text, code string, resolve func(string) string) bool {
	for _, chunk := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '.' || r == '!' || r == '?' }) {
		if len(chunk) > 400 {
			chunk = chunk[:400]
		}
		if resolve(chunk) == code {
			return true
		}
	}
	return false
}

// ambiguousTech are technology names that are also ordinary words, so
// finding the word is no proof of the technology.
var ambiguousTech = map[string]bool{"go": true, "spring": true, "swift": true, "rust": true, "ruby": true, "dart": true, "next": true, "express": true, "r": true, "c": true}

// techContext are words that, near an ambiguous name, show it is a
// technology being asked for.
var techContext = []string{"experience", "proficien", "skills", "stack", "knowledge", "language", "framework", "developer", "engineer",
	"backend", "frontend", "golang", "service", "microservice", "code", "coding", "written in", "built in", "programming", "deneyim", "bilgi", "teknoloji", "geliştirici", "yazılım", "kütüphane"}

// usedAsTechnology reports whether an ambiguous name appears capitalised, as
// a name, with technology wording around it.
func usedAsTechnology(name, original string, accepted []string) bool {
	lower := strings.ToLower(original)
	for from := 0; ; {
		i := strings.Index(lower[from:], name)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(name)
		from = end
		if !boundaryBefore(lower, start) || !boundaryAfter(lower, end) {
			continue
		}
		// Lower-casing can change byte offsets; check the capital only when it did not.
		if len(lower) == len(original) && name != "r" && name != "c" {
			if r, _ := utf8.DecodeRuneInString(original[start:]); !unicode.IsUpper(r) {
				continue
			}
		}
		lo, hi := max(0, start-60), min(len(lower), end+60)
		for lo > 0 && !utf8.RuneStart(lower[lo]) {
			lo--
		}
		for hi < len(lower) && !utf8.RuneStart(lower[hi]) {
			hi++
		}
		window := lower[lo:hi]
		for _, w := range append(slices.Clone(techContext), accepted...) {
			if strings.Contains(window, w) {
				return true
			}
		}
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

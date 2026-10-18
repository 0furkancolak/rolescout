package llm

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// technologyAliases lists other spellings that count as the same technology
// when grounding a name in the text.
var technologyAliases = map[string][]string{
	"kubernetes":          {"k8s"},
	"javascript":          {"js", "ecmascript"},
	"typescript":          {"ts"},
	"go":                  {"golang"},
	"postgresql":          {"postgres"},
	"node.js":             {"nodejs", "node"},
	"react":               {"reactjs", "react.js"},
	"vue":                 {"vuejs", "vue.js"},
	"c#":                  {"csharp", ".net"},
	"c++":                 {"cpp"},
	"mongodb":             {"mongo"},
	"elasticsearch":       {"elastic"},
	"amazon web services": {"aws"},
	"aws":                 {"amazon web services"},
	"gcp":                 {"google cloud"},
}

// containsWord reports whether word occurs in text (both lower case) with no
// letter, digit, '+' or '#' directly on either side.
func containsWord(text, word string) bool {
	if word == "" {
		return false
	}
	for from := 0; ; {
		i := strings.Index(text[from:], word)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(word)
		if boundaryBefore(text, start) && boundaryAfter(text, end) {
			return true
		}
		from = start + 1
		for from < len(text) && !utf8.RuneStart(text[from]) {
			from++
		}
	}
}

// containsPrefixWord is containsWord without the boundary after the word.
func containsPrefixWord(text, stem string) bool {
	for from := 0; from < len(text); {
		i := strings.Index(text[from:], stem)
		if i < 0 {
			return false
		}
		start := from + i
		if boundaryBefore(text, start) {
			return true
		}
		from = start + len(stem)
	}
	return false
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '#'
}

func boundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !isWordRune(r)
}

func boundaryAfter(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return !isWordRune(r)
}

var numberPattern = regexp.MustCompile(`\d[\d.,  ]*\d|\d`)
var kSuffix = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)\s?[kK]\b`)

// numbersIn returns every whole number written in the text, understanding
// thousands separators (1,000 1.000 1 000), and "100k" style shorthand.
func numbersIn(text string) map[int]bool {
	out := map[int]bool{}
	for _, loc := range numberPattern.FindAllStringIndex(text, -1) {
		raw := text[loc[0]:loc[1]]
		add := func(s string) {
			if n, err := strconv.Atoi(s); err == nil {
				out[n] = true
			}
		}
		// k shorthand directly after the digits: "100k", "1.5k".
		if m := kSuffix.FindStringSubmatch(text[loc[0]:]); m != nil {
			if f, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64); err == nil {
				out[int(f*1000+0.5)] = true
			}
		}
		// Whole run with separators removed ("120.000", "80 000", "1,000").
		clean := strings.NewReplacer(".", "", ",", "", " ", "", " ", "", " ", "").Replace(raw)
		add(clean)
		// Each separated chunk on its own ("100,000 - 120,000" is two numbers, and
		// "3.5" is not a thousands number).
		for _, part := range regexp.MustCompile(`[ ,.]+`).Split(raw, -1) {
			add(part)
		}
	}
	return out
}

// cleanText removes control characters and trims.
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		if r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// truncateRunes cuts s to at most max runes, preferring the last space.
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)[:max]
	cut := string(r)
	if i := strings.LastIndexAny(cut, " \n"); i > len(cut)/2 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut)
}

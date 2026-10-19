package ingest

import (
	_ "embed"
	"regexp"
	"strings"
	"unicode"
)

//go:embed places.tsv
var placesTSV string

// folder removes the diacritics of Turkish and common European letters, so
// "İstanbul", "Istanbul" and "istanbul" are the same place and "Türkiye"
// matches "turkiye".
var folder = strings.NewReplacer(
	"İ", "I", "ı", "i", "ç", "c", "Ç", "C", "ğ", "g", "Ğ", "G", "ö", "o", "Ö", "O", "ş", "s", "Ş", "S", "ü", "u", "Ü", "U",
	"â", "a", "Â", "A", "î", "i", "Î", "I", "û", "u", "Û", "U", "é", "e", "è", "e", "ê", "e", "ë", "e", "É", "E",
	"á", "a", "à", "a", "ä", "a", "ã", "a", "å", "a", "ñ", "n", "ó", "o", "ò", "o", "ô", "o", "õ", "o", "í", "i", "ì", "i",
	"ï", "i", "ú", "u", "ù", "u", "ß", "ss", "ø", "o", "æ", "ae", "ł", "l",
)

// Fold lower-cases s and removes diacritics. Search uses it so a query for
// "izmir" finds "İzmir".
func Fold(s string) string { return strings.ToLower(folder.Replace(s)) }

// places maps folded country, region and city names to ISO 3166-1 alpha-2
// codes.
var places = func() map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(placesTSV, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, code, ok := strings.Cut(line, "\t")
		if ok {
			m[Fold(name)] = strings.TrimSpace(code)
		}
	}
	return m
}()

// usStates are abbreviations that, after a comma, mean the United States.
var usStates = map[string]bool{}

func init() {
	for _, s := range strings.Fields("AL AK AZ AR CA CO CT DE FL GA HI ID IL IN IA KS KY LA ME MD MA MI MN MS MO MT NE NV NH NJ NM NY NC ND OH OK OR PA RI SC SD TN TX UT VT VA WA WV WI WY DC") {
		usStates[s] = true
	}
}

var countryCode = regexp.MustCompile(`^[A-Z]{2}$`)

// knownCodes are ISO codes that appear in places.tsv.
var knownCodes = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range places {
		m[c] = true
	}
	return m
}()

// InferCountry guesses the country of a free-text job location, such as
// "Berlin, Germany" or "İstanbul". The last place mentioned wins, so
// "Georgia, US" is the United States. It returns "" when unsure.
func InferCountry(location string) string {
	if strings.TrimSpace(location) == "" {
		return ""
	}
	words := splitWords(location)
	best, bestPos := "", -1
	for i := range words {
		// Multi-word names first: up to four words starting at i.
		for n := min(4, len(words)-i); n >= 1; n-- {
			phrase := strings.ToLower(strings.Join(texts(words[i:i+n]), " "))
			if code, ok := places[phrase]; ok {
				// "US", "UK", "UAE" only count as codes when written in upper case.
				if n == 1 && len(words[i].text) <= 3 && words[i].text != strings.ToUpper(words[i].text) {
					continue
				}
				if words[i].pos >= bestPos {
					best, bestPos = code, words[i].pos
				}
				break
			}
		}
		w := words[i]
		if w.afterComma && countryCode.MatchString(w.text) {
			switch {
			case usStates[w.text] && w.pos >= bestPos:
				best, bestPos = "US", w.pos
			case knownCodes[w.text] && w.pos >= bestPos:
				best, bestPos = w.text, w.pos
			}
		}
	}
	return best
}

type locWord struct {
	text       string
	pos        int
	afterComma bool
}

func splitWords(s string) []locWord {
	s = folder.Replace(s)
	var out []locWord
	start := -1
	afterComma := false
	for i, r := range s + " " {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			out = append(out, locWord{text: s[start:i], pos: start, afterComma: afterComma})
			start = -1
			afterComma = false
		}
		if r == ',' || r == ';' || r == '(' || r == '-' || r == '/' {
			afterComma = true
		}
	}
	return out
}

func texts(ws []locWord) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = w.text
	}
	return out
}

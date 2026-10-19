package ingest

import (
	"slices"
	"strings"
)

// regions groups countries under the names job boards use in locations such
// as "Remote - EMEA" or "Europe". Codes are ISO 3166-1 alpha-2.
var regions = map[string][]string{
	"eu":     strings.Fields("AT BE BG HR CY CZ DK EE FI FR DE GR HU IE IT LV LT LU MT NL PL PT RO SK SI ES SE"),
	"europe": strings.Fields("AL AD AT BY BE BA BG HR CY CZ DK EE FI FR DE GR HU IS IE IT XK LV LI LT LU MT MD MC ME NL MK NO PL PT RO RS SK SI ES SE CH TR UA GB"),
	"emea": strings.Fields("AL AD AT BY BE BA BG HR CY CZ DK EE FI FR DE GR HU IS IE IT LV LT LU MT MD MC ME NL MK NO PL PT RO RS SK SI ES SE CH TR UA GB " +
		"AE SA IL QA KW BH OM JO LB EG MA TN DZ ZA NG KE GH ET"),
	"mena":          strings.Fields("AE SA IL QA KW BH OM JO LB EG MA TN DZ TR IQ IR"),
	"dach":          {"DE", "AT", "CH"},
	"nordics":       strings.Fields("DK FI IS NO SE"),
	"benelux":       {"BE", "NL", "LU"},
	"uk":            {"GB"},
	"north america": {"US", "CA", "MX"},
	"americas":      strings.Fields("US CA MX BR AR CL CO PE UY"),
	"latam":         strings.Fields("MX BR AR CL CO PE UY"),
	"apac":          strings.Fields("AU NZ JP KR SG IN CN HK TW ID MY PH TH VN"),
}

// regionWords are the folded spellings in location text that name a region.
var regionWords = map[string]string{
	"emea": "emea", "europe": "europe", "european union": "eu", "eu": "eu", "mena": "mena", "dach": "dach",
	"nordics": "nordics", "benelux": "benelux", "north america": "north america", "americas": "americas",
	"latam": "latam", "latin america": "latam", "apac": "apac", "asia pacific": "apac", "avrupa": "europe",
}

// RegionNames lists the regions a profile or search can name.
func RegionNames() []string {
	out := make([]string, 0, len(regions))
	for k := range regions {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// RegionCountries returns the countries of a named region ("eu", "emea"...),
// or nil when the name is unknown.
func RegionCountries(name string) []string {
	return slices.Clone(regions[Fold(strings.TrimSpace(name))])
}

// LocationRegions returns the regions a free-text location names, such as
// "Remote - EMEA" or "Europe (remote)".
func LocationRegions(location string) []string {
	words := splitWords(location)
	var out []string
	for i := range words {
		for n := min(2, len(words)-i); n >= 1; n-- {
			phrase := strings.ToLower(strings.Join(texts(words[i:i+n]), " "))
			if r, ok := regionWords[phrase]; ok {
				if !slices.Contains(out, r) {
					out = append(out, r)
				}
				break
			}
		}
	}
	return out
}

// InRegion reports whether location names a region that contains one of the
// countries.
func InRegion(location string, countries []string) bool {
	for _, r := range LocationRegions(location) {
		for _, c := range countries {
			if slices.Contains(regions[r], c) {
				return true
			}
		}
	}
	return false
}

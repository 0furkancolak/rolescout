package discovery

import (
	"net/url"
	"strings"
)

// countryDomains maps a country-code top-level domain to the ISO 3166-1 code
// of the country it stands for. Generic domains (.com, .io, .ai…) say nothing.
var countryDomains = map[string]string{
	"tr": "TR", "de": "DE", "uk": "GB", "fr": "FR", "nl": "NL", "es": "ES", "it": "IT", "pl": "PL", "se": "SE", "no": "NO",
	"dk": "DK", "fi": "FI", "ie": "IE", "pt": "PT", "at": "AT", "ch": "CH", "be": "BE", "cz": "CZ", "ro": "RO", "gr": "GR",
	"ua": "UA", "il": "IL", "in": "IN", "jp": "JP", "kr": "KR", "cn": "CN", "sg": "SG", "au": "AU", "nz": "NZ", "ca": "CA",
	"br": "BR", "mx": "MX", "ar": "AR", "za": "ZA", "ae": "AE", "hu": "HU", "bg": "BG", "hr": "HR", "sk": "SK", "si": "SI",
	"lt": "LT", "lv": "LV", "ee": "EE", "rs": "RS", "us": "US", "pk": "PK", "eg": "EG", "ng": "NG", "ke": "KE", "id": "ID",
	"my": "MY", "ph": "PH", "th": "TH", "vn": "VN", "hk": "HK", "tw": "TW", "cl": "CL", "co": "CO", "pe": "PE",
}

// CountryOfURL returns the country a URL's domain belongs to ("TR" for
// careers.example.com.tr), or "" for a generic domain.
func CountryOfURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	i := strings.LastIndex(host, ".")
	if i < 0 {
		return ""
	}
	return countryDomains[host[i+1:]]
}

// CountryNames are the English names of countries the sector search is asked
// about, by ISO 3166-1 code. A code that is not here is asked about by code.
var CountryNames = map[string]string{
	"TR": "Türkiye", "DE": "Germany", "GB": "United Kingdom", "FR": "France", "NL": "Netherlands", "ES": "Spain", "IT": "Italy",
	"PL": "Poland", "SE": "Sweden", "NO": "Norway", "DK": "Denmark", "FI": "Finland", "IE": "Ireland", "PT": "Portugal",
	"AT": "Austria", "CH": "Switzerland", "BE": "Belgium", "CZ": "Czechia", "RO": "Romania", "GR": "Greece", "UA": "Ukraine",
	"IL": "Israel", "IN": "India", "JP": "Japan", "KR": "South Korea", "CN": "China", "SG": "Singapore", "AU": "Australia",
	"NZ": "New Zealand", "CA": "Canada", "BR": "Brazil", "MX": "Mexico", "AR": "Argentina", "ZA": "South Africa",
	"AE": "United Arab Emirates", "HU": "Hungary", "BG": "Bulgaria", "HR": "Croatia", "US": "United States", "EG": "Egypt",
	"NG": "Nigeria", "KE": "Kenya", "ID": "Indonesia", "MY": "Malaysia", "PH": "Philippines", "TH": "Thailand", "VN": "Vietnam",
	"PK": "Pakistan", "LT": "Lithuania", "LV": "Latvia", "EE": "Estonia", "RS": "Serbia", "SK": "Slovakia", "SI": "Slovenia",
}

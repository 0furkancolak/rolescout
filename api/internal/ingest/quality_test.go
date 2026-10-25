package ingest

import "testing"

func TestIsJunkListing(t *testing.T) {
	for title, junk := range map[string]bool{
		"General Application":                     true,
		"Open application (Find Your Place)":      true,
		"Engineering Talent Community":            true,
		"Join Our Talent Community - SEM/PPC":     true,
		"Spontaneous Application (Tech, Product)": true,
		"https://hover.to":                        true,
		"Remote":                                  true,
		"—":                                       true,
		"Senior Go Engineer":                      false,
		"Application Security Engineer":           false,
		"Open Source Developer Advocate":          false,
		"Community Manager":                       false,
		"Cook":                                    false,
		"デリバリーソリューションアーキテクト": false,
	} {
		if got := IsJunkListing(RawListing{Title: title}); got != junk {
			t.Errorf("IsJunkListing(%q) = %v, want %v", title, got, junk)
		}
	}
}

func TestCleanCompanyName(t *testing.T) {
	for in, want := range map[string]string{
		"Acme":       "Acme",
		"Acme, Inc.": "Acme, Inc.",
		"xAI (Yes, we just got acquired by SpaceX, no, I can't talk about it)":        "xAI",
		"Docker Inc, Remote (62 positions available) We are rapidly expanding":        "Docker Inc",
		"Addepar/ Softare Engineering Manager/ NYC-Hybrid/ Full-time":                 "Addepar",
		"Nectar Social ( https://www.nectarsocial.com )":                              "Nectar Social",
		"Tether - https://tether.io/ - we are hiring for multiple roles 100% remote.": "Tether",
		"Back Market: Senior Full Stack Engineer (Go)- New York - hybrid":             "Back Market",
		"Norm is building AI that understands regulations for regulatory compliance":  "Norm",
		"Dr. Oetker Digital Services GmbH":                                            "Dr. Oetker Digital Services GmbH",
	} {
		if got := CleanCompanyName(in); got != want {
			t.Errorf("CleanCompanyName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlausibleCompanyName(t *testing.T) {
	for name, want := range map[string]bool{
		"Acme": true, "Back Market": true, "Hi friends": false, "Hiring": false, "COMPANY": false, "Senior": false,
		"": false, "We at Avride are hiring in Austin for a full-time position": false, "Foreign Economic Relations Board of Turkey": true,
	} {
		if got := PlausibleCompanyName(name); got != want {
			t.Errorf("PlausibleCompanyName(%q) = %v, want %v", name, got, want)
		}
	}
}

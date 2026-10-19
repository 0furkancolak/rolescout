package ingest

import "testing"

func TestInferCountryFoldsDiacriticsAndKnowsRegionsOfTurkeyAndUS(t *testing.T) {
	for in, want := range map[string]string{
		"Türkiye":                        "TR",
		"İstanbul, Türkiye":              "TR",
		"Kadıköy, İstanbul":              "TR",
		"Şişli / Istanbul (Hybrid)":      "TR",
		"Eskişehir":                      "TR",
		"Long Beach, California":         "US",
		"Mountain View, California (HQ)": "US",
		"Gurugram":                       "IN",
		"Bethesda, Maryland":             "US",
	} {
		if got := InferCountry(in); got != want {
			t.Errorf("InferCountry(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFold(t *testing.T) {
	if got := Fold("İZMİR Çankaya Ğ"); got != "izmir cankaya g" {
		t.Errorf("Fold = %q", got)
	}
}

func TestRegions(t *testing.T) {
	if !InRegion("Remote - EMEA", []string{"TR"}) || !InRegion("Europe (remote)", []string{"DE"}) {
		t.Error("EMEA and Europe should contain TR and DE")
	}
	if InRegion("Remote - EU", []string{"TR"}) || InRegion("Remote, US", []string{"TR"}) || InRegion("", []string{"TR"}) {
		t.Error("EU and plain US should not contain TR")
	}
	if got := RegionCountries("EU"); len(got) != 27 {
		t.Errorf("EU has %d countries", len(got))
	}
	if RegionCountries("narnia") != nil {
		t.Error("unknown region should be nil")
	}
}

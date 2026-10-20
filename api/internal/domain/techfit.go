package domain

import "strings"

// TechFit compares the technologies a job asks for with those a person has
// (their profile's list and their CV).
type TechFit struct {
	// Known is false when there is nothing to compare with: no technologies
	// in the profile and no CV.
	Known bool
	// Matched are the job's technologies the person has.
	Matched []string
	// Missing are the job's technologies the person does not have.
	Missing []string
	// Excluded are missing ones the person ruled out themselves.
	Excluded []string
}

// FitTechnologies compares job technologies with have (profile technologies
// and CV skills) and the profile's excluded keywords. Names are compared
// without case, spaces, dots and dashes, so "Node.js" equals "nodejs".
func FitTechnologies(job, have, excluded []string) TechFit {
	fit := TechFit{Known: len(have) > 0, Matched: []string{}, Missing: []string{}, Excluded: []string{}}
	if !fit.Known {
		return fit
	}
	haveSet := map[string]bool{}
	for _, h := range have {
		haveSet[techKey(h)] = true
	}
	excludedSet := map[string]bool{}
	for _, e := range excluded {
		excludedSet[techKey(e)] = true
	}
	seen := map[string]bool{}
	for _, t := range job {
		k := techKey(t)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		switch {
		case haveSet[k]:
			fit.Matched = append(fit.Matched, t)
		case excludedSet[k]:
			fit.Missing = append(fit.Missing, t)
			fit.Excluded = append(fit.Excluded, t)
		default:
			fit.Missing = append(fit.Missing, t)
		}
	}
	return fit
}

func techKey(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '.', '-', '_', '/':
			return -1
		}
		return r
	}, strings.ToLower(strings.TrimSpace(s)))
}

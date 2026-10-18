package database

import "strings"

// Effective job fields: values from the rules (columns of jobs) always win; the
// optional local model (view job_ai) fills gaps and adds technologies. All SQL
// that filters, counts or scores jobs uses these expressions on the aliases
// "j" (jobs) and "ai" (job_ai joined with aiJoin).
const (
	aiJoin = " LEFT JOIN job_ai ai ON ai.job_id = j.id"

	effSeniority    = "COALESCE(NULLIF(j.seniority, 'unknown'), ai.seniority, 'unknown')"
	effArrangement  = "COALESCE(NULLIF(j.work_arrangement, 'unknown'), ai.work_arrangement, 'unknown')"
	effCountry      = "COALESCE(j.country_code, ai.country_code)"
	effTechnologies = "(j.technologies || COALESCE(ai.technologies, '{}'::text[]))"
	// effTechnologiesDistinct is the same list without duplicates, for display
	// and scoring.
	effTechnologiesDistinct = "ARRAY(SELECT DISTINCT t FROM unnest(" + effTechnologies + ") AS t ORDER BY t)"
)

// effectiveJobColumns is jobColumns for alias j with the effective expressions
// in place of the four rule columns, so a job read through it carries the
// values search and scoring use. The scan order is unchanged.
func effectiveJobColumns() string {
	repl := map[string]string{
		"work_arrangement": effArrangement,
		"seniority":        effSeniority,
		"technologies":     effTechnologiesDistinct,
		"country_code":     effCountry,
	}
	parts := strings.Split(jobColumns, ",")
	for i, c := range parts {
		c = strings.TrimSpace(c)
		if e, ok := repl[c]; ok {
			parts[i] = e
		} else {
			parts[i] = "j." + c
		}
	}
	return strings.Join(parts, ", ")
}

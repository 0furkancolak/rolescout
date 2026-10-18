package llm

import "encoding/json"

// Enrichment is what the model is asked to extract from a job description.
// After Validate every field is either grounded in the text or "unknown"/empty.
type Enrichment struct {
	Technologies    []string `json:"technologies"`
	Seniority       string   `json:"seniority"`
	WorkArrangement string   `json:"work_arrangement"`
	RemotePolicy    string   `json:"remote_policy"`
	EmploymentType  string   `json:"employment_type"`
	SalaryMin       *int     `json:"salary_min"`
	SalaryMax       *int     `json:"salary_max"`
	SalaryCurrency  string   `json:"salary_currency"`
	SalaryPeriod    string   `json:"salary_period"`
	Location        string   `json:"location"`
	CountryCode     string   `json:"country_code"`
	Languages       []string `json:"languages"`
	YearsExperience *int     `json:"years_experience"`
	Summary         string   `json:"summary"`
}

// SchemaVersion changes whenever the Enrichment shape does; it is part of the
// cache key.
const SchemaVersion = 1

var (
	seniorityEnum    = []string{"intern", "junior", "mid", "senior", "lead", "principal", "unknown"}
	arrangementEnum  = []string{"remote", "hybrid", "onsite", "unknown"}
	employmentEnum   = []string{"full_time", "part_time", "contract", "internship", "temporary", "unknown"}
	salaryPeriodEnum = []string{"year", "month", "day", "hour", "unknown"}
)

// JobSchema is the JSON Schema sent with every request. It is closed (strict
// mode: every property required, no extras) so constrained decoding can only
// produce these fields.
func JobSchema() json.RawMessage {
	str := map[string]any{"type": "string"}
	nullableInt := map[string]any{"type": []string{"integer", "null"}}
	props := map[string]any{
		"technologies":     map[string]any{"type": "array", "items": str, "maxItems": 25},
		"seniority":        map[string]any{"type": "string", "enum": seniorityEnum},
		"work_arrangement": map[string]any{"type": "string", "enum": arrangementEnum},
		"remote_policy":    str,
		"employment_type":  map[string]any{"type": "string", "enum": employmentEnum},
		"salary_min":       nullableInt,
		"salary_max":       nullableInt,
		"salary_currency":  str,
		"salary_period":    map[string]any{"type": "string", "enum": salaryPeriodEnum},
		"location":         str,
		"country_code":     str,
		"languages":        map[string]any{"type": "array", "items": str, "maxItems": 5},
		"years_experience": nullableInt,
		"summary":          str,
	}
	required := make([]string, 0, len(props))
	for _, k := range []string{"technologies", "seniority", "work_arrangement", "remote_policy", "employment_type", "salary_min", "salary_max", "salary_currency", "salary_period", "location", "country_code", "languages", "years_experience", "summary"} {
		required = append(required, k)
	}
	b, err := json.Marshal(map[string]any{
		"type": "object", "properties": props, "required": required, "additionalProperties": false,
	})
	if err != nil {
		panic(err) // static data
	}
	return b
}
